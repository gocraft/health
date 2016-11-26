package sentry

import (
	"fmt"
	"os"

	"github.com/getsentry/raven-go"
	"github.com/gocraft/health"
)

// Sink emits errors to Sentry.
type Sink struct {
	Config *Config
	raven  *raven.Client
}

type cmdEventErr struct {
	Job   string
	Event string
	Err   *health.UnmutedError
	Kvs   map[string]string
}

// Config is used to configure Sentry sink.
type Config struct {
	// Application's Sentry URL.
	URL string
	// Only send errors if set.
	ErrorsOnly bool
}

// NewSink creates and returns new Sentry sink
// configured by given config.
func NewSink(config *Config) *Sink {
	const maxChanSize = 25

	epRaven, err := raven.NewClient(config.URL, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sentry.Notify: could not connect to Sentry. err=%v\n", err)
	}
	s := &Sink{
		Config: config,
		raven:  epRaven,
	}

	return s
}

func (s *Sink) EmitEvent(job string, event string, kvs map[string]string) {
	// Ignore events if ErrorsOnly is set
	if s.Config.ErrorsOnly {
		return
	}
	packet := raven.NewPacket(job)
	packet.Level = raven.INFO
	kvs["event"] = event
	s.raven.Capture(packet, kvs)
}

func (s *Sink) EmitEventErr(job string, event string, inputErr error, kvs map[string]string) {
	switch inputErr := inputErr.(type) {
	case *health.UnmutedError:
		if !inputErr.Emitted {
			packet := raven.NewPacket(job, raven.NewException((inputErr), raven.NewStacktrace(2, 3, nil)))
			s.raven.Capture(packet, kvs)
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

func (s *Sink) EmitComplete(job string, status health.CompletionStatus, nanos int64, kvs map[string]string) {
	// no-op
}

func (s *Sink) ShutdownServer() {
	s.raven.Close()
}
