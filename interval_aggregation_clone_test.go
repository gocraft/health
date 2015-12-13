package health

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type eventErr struct {
	event string
	err   error
}

func TestClone(t *testing.T) {
	setNowMock("2011-09-09T23:36:13Z")
	defer resetNowMock()

	a := aggregatorWithData()
	intAgg := a.intervalAggregations[0]
	assertAggregationData(t, intAgg)
	clonedAgg := intAgg.Clone()
	assertAggregationData(t, clonedAgg)

	// Let's add some data to intAgg and make sure it doesn't propagate to clonedAgg
	a.EmitEvent("foo", "bar")
	a.EmitTiming("foo", "bar", 100)
	a.EmitEventErr("foo", "bar", fmt.Errorf("hi"))
	a.EmitGauge("foo", "bar", 3.14)
	a.EmitComplete("foo", Error, 99)

	assert.Equal(t, 301, len(intAgg.Jobs))

	assertAggregationData(t, clonedAgg)
}

func BenchmarkClone(b *testing.B) {
	setNowMock("2011-09-09T23:36:13Z")
	defer resetNowMock()

	a := aggregatorWithData()
	intAgg := a.intervalAggregations[0]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		intAgg.Clone()
	}
}

func assertAggregationData(t *testing.T, intAgg *IntervalAggregation) {
	assert.Equal(t, 300, len(intAgg.Jobs))
	assert.Equal(t, 1200, len(intAgg.Events))
	assert.Equal(t, 1200, len(intAgg.Timers))
	assert.Equal(t, 1200, len(intAgg.Gauges))
	assert.Equal(t, 1200, len(intAgg.EventErrs))

	// Spot-check events:
	assert.EqualValues(t, 1, intAgg.Events["event0"])

	// Spot check gauges:
	assert.EqualValues(t, 3.14, intAgg.Gauges["gauge0"])

	// Spot-check timings:
	assert.EqualValues(t, 1, intAgg.Timers["timing0"].Count)
	assert.EqualValues(t, 12, intAgg.Timers["timing0"].NanosSum)

	// Spot-check event-errs:
	assert.EqualValues(t, 1, intAgg.EventErrs["err0"].Count)
	assert.Equal(t, []error{fmt.Errorf("wat")}, intAgg.EventErrs["err0"].getErrorSamples())

	// Spot-check jobs:
	job := intAgg.Jobs["job0"]
	assert.EqualValues(t, 1, job.CountSuccess)
	assert.EqualValues(t, 0, job.CountError)
	assert.EqualValues(t, 1, job.Events["event0"])
	assert.EqualValues(t, 0, job.Events["event4"])
	assert.EqualValues(t, 3.14, job.Gauges["gauge0"])
	assert.EqualValues(t, 0.0, job.Gauges["gauge4"])
	assert.EqualValues(t, 1, job.Timers["timing0"].Count)
	assert.EqualValues(t, 12, job.Timers["timing0"].NanosSum)
	assert.EqualValues(t, 1, job.EventErrs["err0"].Count)
	assert.Equal(t, []error{fmt.Errorf("wat")}, job.EventErrs["err0"].getErrorSamples())

	// Nothing foo or bar related
	_, ok := intAgg.Jobs["foo"]
	assert.False(t, ok)
	assert.EqualValues(t, 0, intAgg.Events["bar"])
	assert.Nil(t, intAgg.Timers["bar"])
	assert.Nil(t, intAgg.EventErrs["bar"])

}

func aggregatorWithData() *aggregator {
	a := newAggregator(time.Minute, time.Minute*5)

	// We want 300 jobs
	// Each job will have 5 events, but we want 1200 events total
	// Each job will have 5 timers, but we want 1200 timers total
	// Each job will have 5 gauges, but we want 1200 gauges total
	// Each job will have 5 errs, but we want 1200 errs total
	// Given this 300/1200 dichotomy,
	//  - the first job will have 4 events, the next job 4 events, etc.

	jobs := []string{}
	for i := 0; i < 300; i++ {
		jobs = append(jobs, fmt.Sprintf("job%d", i))
	}

	events := []string{}
	for i := 0; i < 1200; i++ {
		events = append(events, fmt.Sprintf("event%d", i))
	}

	timings := []string{}
	for i := 0; i < 1200; i++ {
		timings = append(timings, fmt.Sprintf("timing%d", i))
	}

	gauges := []string{}
	for i := 0; i < 1200; i++ {
		gauges = append(gauges, fmt.Sprintf("gauge%d", i))
	}

	eventErrs := []eventErr{}
	for i := 0; i < 1200; i++ {
		eventErrs = append(eventErrs, eventErr{
			event: fmt.Sprintf("err%d", i),
			err:   fmt.Errorf("wat"),
		})
	}

	cur := 0
	for _, j := range jobs {
		for i := 0; i < 4; i++ {
			a.EmitEvent(j, events[cur])
			cur++
		}
	}

	cur = 0
	for _, j := range jobs {
		for i := 0; i < 4; i++ {
			a.EmitEventErr(j, eventErrs[cur].event, eventErrs[cur].err)
			cur++
		}
	}

	cur = 0
	for _, j := range jobs {
		for i := 0; i < 4; i++ {
			a.EmitTiming(j, timings[cur], 12)
			cur++
		}
	}

	cur = 0
	for _, j := range jobs {
		for i := 0; i < 4; i++ {
			a.EmitGauge(j, gauges[cur], 3.14)
			cur++
		}
	}

	for _, j := range jobs {
		a.EmitComplete(j, Success, 12)
	}

	return a
}
