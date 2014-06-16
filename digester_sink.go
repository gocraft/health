package health

import (
	"time"
)

type DigesterSink struct {
	NewEvents chan *metric
}

// a metric unifies the arguments to the Emit* functions.
// The kvs are opaque and only for logging, so those are omitted
type metric struct {
	Kind metricKind
	Job string
	Event string
	Err string
	Nanos int64
	Status CompletionStatus
}

type metricKind int
const (
	metricKindEvent metricKind = iota
	metricKindErr
	metricKindTiming
	metricKindComplete
)

func NewDigesterSink() *DigesterSink {
	sink := &DigesterSink{
		NewEvents: make(chan *metric, 1024),
	}
	go sink.digest()
	
	return sink
}

type jobStats struct {
	Name string
}

// Important Questions:
// - Job-focused stats:
//   - Given Job, tell me things about it:
//   - Avg # of each event per job (can we get a histogram for this?)
//   - 

func (s *DigesterSink) digest() {
	tick := time.Tick(10 * time.Second)
	
	var counters map[string]int64
	var jobs map[string]*jobStats
	
	for {
		select {
		case <-tick:
			// wat
		case m := <-s.NewEvents:
			if m.Kind == metricKindEvent {
				// This is really 2 events:
				// 1) event
				// 2) job.event
			}
		}
	}
}


func (s *DigesterSink) EmitEvent(job string, event string, kvs map[string]string) error {
	s.NewEvents <- &metric{
		Kind: metricKindEvent,
		Job: job,
		Event: event,
	}
	return nil
}

func (s *DigesterSink) EmitEventErr(job string, event string, inputErr error, kvs map[string]string) error {
	s.NewEvents <- &metric{
		Kind: metricKindErr,
		Job: job,
		Event: event,
		Err: inputErr.Error(),
	}
	return nil
}

func (s *DigesterSink) EmitTiming(job string, event string, nanos int64, kvs map[string]string) error {
	s.NewEvents <- &metric{
		Kind: metricKindTiming,
		Job: job,
		Event: event,
		Nanos: nanos,
	}
	return nil
}

func (s *DigesterSink) EmitComplete(job string, status CompletionStatus, nanos int64, kvs map[string]string) error {
	s.NewEvents <- &metric{
		Kind: metricKindComplete,
		Job: job,
		Status: status,
		Nanos: nanos,
	}
	return nil
}