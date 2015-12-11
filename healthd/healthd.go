package healthd

import (
	"fmt"
	"github.com/gocraft/health"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type HealthD struct {
	stream *health.Stream

	// How long is each aggregation interval. Eg, 1 minute
	intervalDuration time.Duration

	// Retain controls how many metrics interval we keep. Eg, 5 minutes
	retain time.Duration

	// maxIntervals is the maximum length of intervals.
	// It is retain / interval.
	maxIntervals int

	// These guys are the real aggregated deal
	intervalAggregations []*health.IntervalAggregation

	// let's keep the last 5 minutes worth of data from each host
	hostAggregations map[hostAggregationKey]*health.IntervalAggregation

	// intervalsNeedingRecalculation is a set of intervals that need to be recalculated. It is cleared when they are recalculated.
	intervalsNeedingRecalculation map[time.Time]struct{}

	// map from HostPort to status
	hostStatus map[string]*HostStatus

	intervalsChanChan chan chan []*health.IntervalAggregation
	hostsChanChan     chan chan []*HostStatus

	stopFlag           int64
	stopAggregator     chan bool
	stopStopAggregator chan bool
	stopHTTP           func() bool
}

type HostStatus struct {
	HostPort string `json:"host_port"`

	LastCheckTime        time.Time     `json:"last_check_time"`
	LastInstanceId       string        `json:"last_instance_id"`
	LastIntervalDuration time.Duration `json:"last_interval_duration"`
	LastErr              string        `json:"last_err"`
	LastNanos            int64         `json:"last_nanos"`
	LastCode             int           `json:"last_code"` // http status code of last response

	FirstSuccessfulResponse time.Time `json:"first_successful_response"`
	LastSuccessfulResponse  time.Time `json:"last_successful_response"`
}

type hostAggregationKey struct {
	Time       time.Time
	InstanceId string
	HostPort   string
}

func StartNewHealthD(monitoredHostPorts []string, serverHostPort string, stream *health.Stream) *HealthD {
	hd := &HealthD{}
	hd.stream = stream
	hd.intervalsChanChan = make(chan chan []*health.IntervalAggregation, 16)
	hd.hostsChanChan = make(chan chan []*HostStatus, 16)
	hd.hostStatus = make(map[string]*HostStatus)
	hd.hostAggregations = make(map[hostAggregationKey]*health.IntervalAggregation)
	hd.intervalsNeedingRecalculation = make(map[time.Time]struct{})
	hd.retain = time.Hour * 2 // In the future this should be configurable
	hd.intervalDuration = 0   // We don't know this yet. Will be configured from polled hosts.
	hd.maxIntervals = 0       // We don't know this yet. See above.
	hd.stopAggregator = make(chan bool)
	hd.stopStopAggregator = make(chan bool)

	for _, hp := range monitoredHostPorts {
		hd.hostStatus[hp] = &HostStatus{
			HostPort: hp,
		}
	}

	go hd.pollAndAggregate()

	httpStarted := make(chan bool)
	go hd.startHttpServer(serverHostPort, httpStarted)
	<-httpStarted

	return hd
}

func (hd *HealthD) Stop() {
	atomic.StoreInt64(&hd.stopFlag, 1)
	hd.stopAggregator <- true
	<-hd.stopStopAggregator
	hd.stopHTTP()
}

// shouldStop returns true if we've been flagged to stop
func (hd *HealthD) shouldStop() bool {
	v := atomic.LoadInt64(&hd.stopFlag)
	return v == 1
}

func (hd *HealthD) pollAndAggregate() {
	ticker := time.Tick(10 * time.Second)

	responses := make(chan *pollResponse, 64)
	recalcIntervals := make(chan struct{})
	recalcIntervalsRequest := make(chan struct{}, 64)
	intervalsChanChan := hd.intervalsChanChan
	hostsChanChan := hd.hostsChanChan

	go debouncer(recalcIntervals, recalcIntervalsRequest, time.Second*2, time.Millisecond*300)

	// Immediately poll for servers on healthd startup
	go hd.poll(responses)

AGGREGATE_LOOP:
	for {
		// Usual flow:
		// 1. ticker ticks. Poll each host.
		// 2. Get responses in. Trigger debouncer
		// 3. If we get all responses quickly, we'll get a nil, and then recalc.
		// 4. The debouncer will fire in 2 seconds and do a partial calc or full recalc.
		// 5. Repeat 2-4 until all resonses are in and everything settles down.
		// At any time, we could get:
		//  - A requset for metrics. We'll get a channel and send response back on that channel.
		//  - A requset to shut down.
		select {
		case <-ticker:
			go hd.poll(responses)
			hd.purge()
		case resp := <-responses:
			if resp == nil {
				// nil is a sentinel value that is sent when all hosts have reported in.
				hd.recalculateIntervals()
			} else {
				hd.consumePollResponse(resp)
				recalcIntervalsRequest <- struct{}{}
			}
		case <-recalcIntervals:
			hd.recalculateIntervals()
		case intervalsChan := <-intervalsChanChan:
			intervalsChan <- hd.memorySafeIntervals()
		case hostsChan := <-hostsChanChan:
			hostsChan <- hd.memorySafeHosts()
		case <-hd.stopAggregator:
			hd.stopStopAggregator <- true
			break AGGREGATE_LOOP
		}
	}
}

// poll is meant to be alled in a new goroutine.
// It will poll each managed host in a new goroutine.
// When everything has finished, it will send nil to responses to signal that we have all data.
func (hd *HealthD) poll(responses chan *pollResponse) {
	var wg sync.WaitGroup
	for _, hs := range hd.hostStatus {
		wg.Add(1)
		go func(hs *HostStatus) {
			defer wg.Done()
			poll(hd.stream, hs.HostPort, responses)
		}(hs)
	}
	wg.Wait()
	responses <- nil
}

func (hd *HealthD) getAggregationSequence() []*health.IntervalAggregation {
	if hd.shouldStop() {
		return nil
	}
	intervalsChan := make(chan []*health.IntervalAggregation)
	hd.intervalsChanChan <- intervalsChan
	return <-intervalsChan
}

func (hd *HealthD) getHosts() []*HostStatus {
	if hd.shouldStop() {
		return nil
	}
	hostsChan := make(chan []*HostStatus)
	hd.hostsChanChan <- hostsChan
	return <-hostsChan
}

func (agg *HealthD) memorySafeIntervals() []*health.IntervalAggregation {
	ret := make([]*health.IntervalAggregation, 0, len(agg.intervalAggregations))

	for _, intAgg := range agg.intervalAggregations {
		ret = append(ret, intAgg.Clone())
	}

	return ret
}

func (hd *HealthD) memorySafeHosts() []*HostStatus {
	ret := make([]*HostStatus, 0, len(hd.hostStatus))

	for _, hs := range hd.hostStatus {
		var host = *hs // copy mem
		ret = append(ret, &host)
	}

	return ret
}

func (hd *HealthD) consumePollResponse(resp *pollResponse) {
	if hs, ok := hd.hostStatus[resp.HostPort]; ok {
		hs.LastCheckTime = resp.Timestamp
		hs.LastNanos = resp.Nanos
		hs.LastInstanceId = resp.InstanceId
		hs.LastIntervalDuration = resp.IntervalDuration
		hs.LastCode = resp.Code
		if resp.Err == nil {
			hs.LastErr = ""
		} else {
			hs.LastErr = resp.Err.Error()
		}

		if resp.Code == 200 && resp.Err == nil {
			if hs.FirstSuccessfulResponse.IsZero() {
				hs.FirstSuccessfulResponse = now()
			}
			hs.LastSuccessfulResponse = now()
		}
	} else {
		// BUG
		// TODO: log that we got an unknown hostPort.
	}

	// Add resp to hostAggregations
	if resp.Code == 200 && resp.Err == nil {
		if hd.intervalDuration == 0 {
			hd.intervalDuration = resp.IntervalDuration // TODO: validate this
			hd.maxIntervals = int(hd.retain / hd.intervalDuration)
		} else if hd.intervalDuration != resp.IntervalDuration {
			fmt.Println("interval duration mismatch: agg.intervalDuration=", hd.intervalDuration, " but resp.IntervalDuration=", resp.IntervalDuration)
			return
		}

		for _, intAgg := range resp.IntervalAggregations {
			key := hostAggregationKey{
				Time:       intAgg.IntervalStart,
				InstanceId: resp.InstanceId,
				HostPort:   resp.HostPort,
			}

			existingIntAgg, ok := hd.hostAggregations[key]
			if ok && existingIntAgg.SerialNumber == intAgg.SerialNumber {
				// ignore; we already have this data
			} else {
				hd.hostAggregations[key] = intAgg
				hd.intervalsNeedingRecalculation[intAgg.IntervalStart] = struct{}{}
			}
		}
	}
}

// purge purges old hostAggregations older than 5 intervals
func (agg *HealthD) purge() {
	var threshold = agg.intervalDuration * 5 // NOTE: this is arbitrary.
	for k, _ := range agg.hostAggregations {
		if time.Since(k.Time) > threshold {
			delete(agg.hostAggregations, k)
		}
	}

	n := len(agg.intervalAggregations)
	if n > agg.maxIntervals {
		agg.intervalAggregations = agg.intervalAggregations[(n - agg.maxIntervals):]
	}
}

func (hd *HealthD) recalculateIntervals() {
	job := hd.stream.NewJob("recalculate")

	for k, _ := range hd.intervalsNeedingRecalculation {
		intAggsAtTime := []*health.IntervalAggregation{}

		for key, intAgg := range hd.hostAggregations {
			if key.Time == k {
				intAggsAtTime = append(intAggsAtTime, intAgg)
			}
		}

		overallAgg := health.NewIntervalAggregation(k)
		for _, ia := range intAggsAtTime {
			overallAgg.Merge(ia)
		}
		hd.setAggregation(overallAgg)
	}

	// Reset everything:
	hd.intervalsNeedingRecalculation = make(map[time.Time]struct{})

	job.Complete(health.Success)
}

type ByInterval []*health.IntervalAggregation

func (a ByInterval) Len() int           { return len(a) }
func (a ByInterval) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByInterval) Less(i, j int) bool { return a[i].IntervalStart.Before(a[j].IntervalStart) }

func (agg *HealthD) setAggregation(intAgg *health.IntervalAggregation) {
	// If we already have the intAgg, replace it.
	for i, existingAgg := range agg.intervalAggregations {
		if existingAgg.IntervalStart == intAgg.IntervalStart {
			agg.intervalAggregations[i] = intAgg
			return
		}
	}

	// Otherwise, just append it and sort to get ordering right.
	agg.intervalAggregations = append(agg.intervalAggregations, intAgg)
	sort.Sort(ByInterval(agg.intervalAggregations))

	// If we have too many aggregations, truncate
	n := len(agg.intervalAggregations)
	if n > agg.maxIntervals {
		agg.intervalAggregations = agg.intervalAggregations[(n - agg.maxIntervals):]
	}
}
