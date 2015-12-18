package librato

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gocraft/health"
)

type SanitizationFunc func(string) string

type Sink struct {
	SanitizationFunc
	Source      string
	FlushPeriod time.Duration

	cmdChan      chan *emitCmd
	doneChan     chan int
	doneDoneChan chan int
	httpClient   *http.Client

	libratoUser   string
	libratoApiKey string

	// Prefix is something like "metroid"
	// Events emitted to StatsD would be metroid.myevent.wat
	// Eg, don't include a trailing dot in the prefix.
	// It can be "", that's fine.
	prefix string

	timers   map[string]*gauge
	counters map[string]int64
}

type gauge struct {
	Count      int64           `json:"count"`
	Sum        float64         `json:"sum"`
	Min        float64         `json:"min"`
	Max        float64         `json:"max"`
	SumSquares float64         `json:"sum_squares"`
	Attributes gaugeAttributes `json:"attributes"`
}

type gaugeAttributes struct {
	Aggregate         bool   `json:"aggregate"`
	DisplayUnitsShort string `json:"display_units_long"`
}

type libratoCounterValue struct {
	Value      int64           `json:"value"`
	Attributes gaugeAttributes `json:"attributes"`
}

type libratoMetricsPost struct {
	MeasureTime int64                  `json:"measure_time"`
	Period      int64                  `json:"period"`
	Source      string                 `json:"source,omitempty"`
	Gauges      map[string]interface{} `json:"gauges,omitempty"`
}

var defaultTimerAttributes gaugeAttributes = gaugeAttributes{true, "ms"}
var defaultCounterAttributes gaugeAttributes = gaugeAttributes{true, "count"}
var libratoRequestPath string = "https://metrics-api.librato.com/v1/metrics"

type cmdKind int

const (
	cmdKindEvent cmdKind = iota
	cmdKindEventErr
	cmdKindTiming
	cmdKindGauge
	cmdKindComplete
)

type emitCmd struct {
	Kind   cmdKind
	Job    string
	Event  string
	Err    error
	Nanos  int64
	Value  float64
	Status health.CompletionStatus
}

func New(user, apiKey, prefix string) *Sink {
	const buffSize = 4096 // random-ass-guess

	s := &Sink{
		SanitizationFunc: sanitizeKey,
		FlushPeriod:      15 * time.Second,
		cmdChan:          make(chan *emitCmd, buffSize),
		doneChan:         make(chan int),
		doneDoneChan:     make(chan int),
		httpClient:       &http.Client{},
		libratoUser:      user,
		libratoApiKey:    apiKey,
		prefix:           prefix,
		timers:           make(map[string]*gauge),
		counters:         make(map[string]int64),
	}

	s.Source, _ = os.Hostname()

	go s.start()

	return s
}

func (s *Sink) Stop() {
	s.doneChan <- 1
	<-s.doneDoneChan
}

func (s *Sink) EmitEvent(job string, event string, kvs map[string]string) {
	s.cmdChan <- &emitCmd{Kind: cmdKindEvent, Job: job, Event: event}
}

func (s *Sink) EmitEventErr(job string, event string, inputErr error, kvs map[string]string) {
	s.cmdChan <- &emitCmd{Kind: cmdKindEventErr, Job: job, Event: event, Err: inputErr}
}

func (s *Sink) EmitTiming(job string, event string, nanos int64, kvs map[string]string) {
	s.cmdChan <- &emitCmd{Kind: cmdKindTiming, Job: job, Event: event, Nanos: nanos}
}

func (s *Sink) EmitGauge(job string, event string, value float64, kvs map[string]string) {
	s.cmdChan <- &emitCmd{Kind: cmdKindGauge, Job: job, Event: event, Value: value}
}

func (s *Sink) EmitComplete(job string, status health.CompletionStatus, nanos int64, kvs map[string]string) {
	s.cmdChan <- &emitCmd{Kind: cmdKindComplete, Job: job, Status: status, Nanos: nanos}
}

func (s *Sink) start() {
	cmdChan := s.cmdChan
	doneChan := s.doneChan
	ticker := time.Tick(s.FlushPeriod)

LIBRATO_LOOP:
	for {
		select {
		case <-doneChan:
			s.doneDoneChan <- 1
			break LIBRATO_LOOP
		case cmd := <-cmdChan:
			if cmd.Kind == cmdKindEvent {
				s.processEvent(cmd.Job, cmd.Event)
			} else if cmd.Kind == cmdKindEventErr {
				s.processEventErr(cmd.Job, cmd.Event, cmd.Err)
			} else if cmd.Kind == cmdKindTiming {
				s.processTiming(cmd.Job, cmd.Event, cmd.Nanos)
			} else if cmd.Kind == cmdKindGauge {
				s.processGauge(cmd.Job, cmd.Event, cmd.Value)
			} else if cmd.Kind == cmdKindComplete {
				s.processComplete(cmd.Job, cmd.Status, cmd.Nanos)
			}
		case <-ticker:
			s.purge()
		}
	}
}

func (s *Sink) processEvent(job string, event string) {
	key1, key2 := s.eventKeys(job, event, "count")
	s.inc(key1)
	s.inc(key2)
}

func (s *Sink) processEventErr(job string, event string, err error) {
	key1, key2 := s.eventKeys(job, event, "error.count")
	s.inc(key1)
	s.inc(key2)
}

func (s *Sink) processTiming(job string, event string, nanos int64) {
	key1, key2 := s.eventKeys(job, event, "timing")
	ms := float64(nanos) / float64(time.Millisecond)
	s.measure(key1, ms)
	s.measure(key2, ms)
}

func (s *Sink) processGauge(job string, event string, value float64) {
	key1, key2 := s.eventKeys(job, event, "gauge")
	s.measure(key1, value)
	s.measure(key2, value)
}

func (s *Sink) processComplete(job string, status health.CompletionStatus, nanos int64) {
	var b bytes.Buffer

	if s.prefix != "" {
		b.WriteString(s.prefix)
		b.WriteRune('.')
	}
	b.WriteString(s.SanitizationFunc(job))
	b.WriteRune('.')
	b.WriteString(status.String())
	b.WriteString(".timing")

	ms := float64(nanos) / float64(time.Millisecond)
	s.measure(b.String(), ms)
}

func (s *Sink) eventKeys(job, event, suffix string) (string, string) {
	var key1 bytes.Buffer // event
	var key2 bytes.Buffer // job.event

	if s.prefix != "" {
		key1.WriteString(s.prefix)
		key1.WriteRune('.')
		key2.WriteString(s.prefix)
		key2.WriteRune('.')
	}

	key1.WriteString(s.SanitizationFunc(event))
	key2.WriteString(s.SanitizationFunc(job))
	key2.WriteRune('.')
	key2.WriteString(s.SanitizationFunc(event))

	if suffix != "" {
		key1.WriteRune('.')
		key1.WriteString(suffix)
		key2.WriteRune('.')
		key2.WriteString(suffix)
	}

	return key1.String(), key2.String()
}

func (s *Sink) inc(key string) {
	s.counters[key] += 1
}

func (s *Sink) measure(key string, value float64) {
	g, ok := s.timers[key]
	if !ok {
		g = &gauge{Min: value, Max: value, Sum: value, Count: 1, SumSquares: value * value, Attributes: defaultTimerAttributes}
		s.timers[key] = g
	} else {
		g.Count++
		g.Sum += value
		g.SumSquares += value * value

		if value < g.Min {
			g.Min = value
		}
		if value > g.Max {
			g.Max = value
		}
	}
}

func (s *Sink) purge() {
	if err := s.send(); err != nil {
		fmt.Println("Error sending to librato: ", err)
	}
	s.timers = make(map[string]*gauge)
	s.counters = make(map[string]int64)
}

func (s *Sink) send() error {

	// no data? don't send anything to librato
	if len(s.timers) == 0 && len(s.counters) == 0 {
		return nil
	}

	body := libratoMetricsPost{
		MeasureTime: time.Now().Unix(),
		Period:      int64(s.FlushPeriod / time.Second),
		Source:      s.Source,
	}

	gauges := make(map[string]interface{})

	for k, v := range s.timers {
		gauges[k] = v
	}

	for k, v := range s.counters {
		gauges[k] = libratoCounterValue{v, defaultCounterAttributes}
	}
	body.Gauges = gauges

	b, err := json.Marshal(body)
	if nil != err {
		return err
	}

	fmt.Println(string(b))

	req, err := http.NewRequest(
		"POST",
		libratoRequestPath,
		bytes.NewBuffer(b),
	)
	if nil != err {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.SetBasicAuth(s.libratoUser, s.libratoApiKey)
	_, err = s.httpClient.Do(req)

	//fmt.Println(resp.Status)

	return err
}

// valid librato charactors: A-Za-z0-9.:-_
func shouldSanitize(r rune) bool {
	switch {
	case 'A' <= r && r <= 'Z':
		fallthrough
	case 'a' <= r && r <= 'z':
		fallthrough
	case '0' <= r && r <= '9':
		fallthrough
	case r == '.':
		fallthrough
	case r == ':':
		fallthrough
	case r == '-':
		fallthrough
	case r == '_':
		return false
	}
	return true
}

func sanitizeKey(k string) string {
	for _, r := range k {
		if shouldSanitize(r) {
			goto SANITIZE
		}
	}
	return k
SANITIZE:
	var key bytes.Buffer
	for _, r := range k {
		if shouldSanitize(r) {
			key.WriteRune('_')
		} else {
			key.WriteRune(r)
		}
	}
	return key.String()
}
