package prometheus

import (
	"time"

	"github.com/gocraft/health"

	"github.com/prometheus/client_golang/prometheus"
)

type Config struct {
	// The Endpoint to notify about crashes. This defaults to
	// "https://notify.bugsnag.com/", if you're using Bugsnag Enterprise then
	// set it to your internal Bugsnag endpoint.
	Endpoint string

	// The hostname of the current server. This defaults to the return value of
	// os.Hostname() and is graphed in the Bugsnag dashboard.
	Hostname string

	// Job name
	Job string

	// Interval push
	Interval time.Duration

	// Prometheus CountersVec. key is built by `job + event`. Required for predefined Counters.
	CounterVecs   map[string]*prometheus.CounterVec
	HistogramVecs map[string]*prometheus.HistogramVec
}

// This sink emits to a Prometheus pushgateway daemon
type Sink struct {
	*Config
	doneChan     chan bool
	doneDoneChan chan bool
}

func NewSink(config *Config) *Sink {

	if config.Endpoint == "" {
		config.Endpoint = "http://localhost:9091/"
	}

	if config.Job == "" {
		config.Job = "app"
	}

	if config.Interval == 0 {
		config.Interval = time.Minute
	}

	if config.CounterVecs == nil {
		config.CounterVecs = map[string]*prometheus.CounterVec{}
	}

	if config.HistogramVecs == nil {
		config.HistogramVecs = map[string]*prometheus.HistogramVec{}
	}

	s := &Sink{Config: config, doneChan: make(chan bool), doneDoneChan: make(chan bool)}

	go processingLoop(s)

	return s
}

func (s *Sink) EmitEvent(job string, event string, kvs map[string]string) {
	var counter *prometheus.CounterVec
	if c, ok := s.Config.CounterVecs[job]; ok {
		counter = c
	} else {
		labels := labelsFromMap(kvs)

		counter = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: s.Config.Job,
			Subsystem: "indexer",
			Name:      job,
			Help:      "Automaticaly created event",
		}, labels)

		s.Config.CounterVecs[job] = counter
		prometheus.Register(counter)
	}

	kvs["event"] = event

	if m, err := counter.GetMetricWith(kvs); err == nil {
		m.Inc()
	}
}

func (s *Sink) EmitEventErr(job string, event string, inputErr error, kvs map[string]string) {
	// no-op
}

func (s *Sink) EmitTiming(job string, event string, nanos int64, kvs map[string]string) {
	s.emitHistogram(job, event, nanos, kvs)
}

func (s *Sink) EmitGauge(job string, event string, value float64, kvs map[string]string) {
	// TODO: implement this
	// (prometheus is a contributed library so I can't easily test it)
}

func (s *Sink) EmitComplete(job string, status health.CompletionStatus, nanos int64, kvs map[string]string) {
	s.emitHistogram(job, status.String(), nanos, kvs)
}

func (s *Sink) ShutdownServer() {
	s.doneChan <- true
	<-s.doneDoneChan
}

// Push current metrics each minute
func processingLoop(sink *Sink) {
	c := time.Tick(sink.Config.Interval)

PROMETHEUS_PROCESSING_LOOP:
	for {
		select {
		case <-c:
			prometheus.Push(sink.Config.Job, sink.Config.Hostname, sink.Config.Endpoint)
		case <-sink.doneChan:
			sink.doneDoneChan <- true
			break PROMETHEUS_PROCESSING_LOOP
		}
	}
}

func (s *Sink) emitHistogram(job string, event string, nanos int64, kvs map[string]string) {
	var counter *prometheus.HistogramVec
	if c, ok := s.Config.HistogramVecs[job]; ok {
		counter = c
	} else {
		labels := labelsFromMap(kvs)

		counter = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: s.Config.Job,
			Subsystem: "duration",
			Name:      job,
			Help:      "Automaticaly created event",
		}, labels)

		s.Config.HistogramVecs[job] = counter
		prometheus.Register(counter)
	}

	kvs["event"] = event

	if m, err := counter.GetMetricWith(kvs); err == nil {
		m.Observe(float64(nanos))
	}
}

func labelsFromMap(kvs map[string]string) []string {
	result := make([]string, 0, len(kvs)+1)
	result = append(result, "event")
	for k := range kvs {
		result = append(result, k)
	}
	return result
}
