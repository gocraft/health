package runtime_metrics

import (
	"testing"
)

type testReceiver struct {
	gauges map[string]float64
}

func TestRuntimeMetrics(t *testing.T) {
	tr := &testReceiver{
		gauges: make(map[string]float64),
	}
	m := NewRuntimeMetrics(tr, nil)
	m.Start()
	defer m.Stop()
	m.Report()

	expectedKeys := []string{"heap_objects", "alloc", "num_gc", "next_gc", "gc_cpu_fraction", "pause_total_ns", "gc_pause_quantile_50", "gc_pause_quantile_max", "num_cgo_call", "num_goroutines", "num_fds_used"}

	for _, k := range expectedKeys {
		if _, ok := tr.gauges[k]; !ok {
			t.Errorf("expected to have key %s but didn't. map=%v", k, tr.gauges)
		}
	}
}

func (t *testReceiver) Event(eventName string) {

}

func (t *testReceiver) EventKv(eventName string, kvs map[string]string) {

}

func (t *testReceiver) EventErr(eventName string, err error) error {
	return nil
}

func (t *testReceiver) EventErrKv(eventName string, err error, kvs map[string]string) error {
	return nil
}

func (t *testReceiver) Timing(eventName string, nanoseconds int64) {

}

func (t *testReceiver) TimingKv(eventName string, nanoseconds int64, kvs map[string]string) {

}

func (t *testReceiver) Gauge(eventName string, value float64) {
	t.gauges[eventName] = value
}

func (t *testReceiver) GaugeKv(eventName string, value float64, kvs map[string]string) {

}
