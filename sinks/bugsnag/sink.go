package bugsnag

import (
	"fmt"
	"github.com/gocraft/health"
	"os"
)

// This sink emits to a StatsD deaemon by sending it a UDP packet.
type Sink struct {
	*Config
	cmdChan  chan *cmdEventErr
	doneChan chan int
}

type cmdEventErr struct {
	Job   string
	Event string
	Err   *health.UnmutedError
	Kvs   map[string]string
}

func NewSink(config *Config) *Sink {
	const maxChanSize = 25

	if config.Endpoint == "" {
		config.Endpoint = "https://notify.bugsnag.com/"
	}

	s := &Sink{
		Config:   config,
		cmdChan:  make(chan *cmdEventErr, maxChanSize),
		doneChan: make(chan int),
	}

	go errorProcessingLoop(s)

	return s
}

func (s *Sink) EmitEvent(job string, event string, kvs map[string]string) {
	// no-op
}

func (s *Sink) EmitEventErr(job string, event string, inputErr error, kvs map[string]string) {
	switch inputErr := inputErr.(type) {
	case *health.UnmutedError:
		if !inputErr.Emitted {
			s.cmdChan <- &cmdEventErr{Job: job, Event: event, Err: inputErr, Kvs: kvs}
		}
	case *health.MutedError:
	// Do nothing!
	default: // eg, case error:
		// This shouldn't happen, all errors passed in here should be wrapped.
	}
}

func (s *Sink) EmitTiming(job string, event string, nanos int64, kvs map[string]string) {
	// no-op
}

func (s *Sink) EmitGauge(job string, event string, value float64, kvs map[string]string) {
	// no-op
}

func (s *Sink) EmitComplete(job string, status health.CompletionStatus, nanos int64, kvs map[string]string) {
	// no-op
}

func (s *Sink) ShutdownServer() {
	s.doneChan <- 1
}

func errorProcessingLoop(sink *Sink) {
	cmdChan := sink.cmdChan
	doneChan := sink.doneChan

PROCESSING_LOOP:
	for {
		select {
		case <-doneChan:
			break PROCESSING_LOOP
		case cmd := <-cmdChan:
			if err := Notify(sink.Config, cmd.Job, cmd.Event, cmd.Err, cmd.Err.Stack, cmd.Kvs); err != nil {
				fmt.Fprintf(os.Stderr, "bugsnag.Notify: could not notify bugsnag. err=%v\n", err)
			}
		}
	}
}
