package health

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Let's leverage clone's fixture data and make sure we can merge into a new blank aggregation to get the same data.
func TestMergeBasic(t *testing.T) {
	setNowMock("2011-09-09T23:36:13Z")
	defer resetNowMock()

	a := aggregatorWithData()
	intAgg := a.intervalAggregations[0]
	assertAggregationData(t, intAgg)
	newAgg := NewIntervalAggregation(intAgg.IntervalStart)
	newAgg.Merge(intAgg)
	assertAggregationData(t, newAgg)
}

func TestMerge(t *testing.T) {
	setNowMock("2011-09-09T23:36:13Z")
	defer resetNowMock()

	// Make two aggregations, merge together:
	a := aggregatorWithData()
	intAgg := a.intervalAggregations[0]
	a2 := aggregatorWithData()
	intAgg2 := a2.intervalAggregations[0]

	// Modify a gauge:
	a2.EmitGauge("job0", "gauge1", 5.5)

	intAgg.Merge(intAgg2)

	// same number of events:
	assert.Equal(t, 300, len(intAgg.Jobs))
	assert.Equal(t, 1200, len(intAgg.Events))
	assert.Equal(t, 1200, len(intAgg.Timers))
	assert.Equal(t, 1200, len(intAgg.Gauges))
	assert.Equal(t, 1200, len(intAgg.EventErrs))

	// Spot-check events:
	assert.EqualValues(t, 2, intAgg.Events["event0"])

	// Spot-check gauges:
	assert.EqualValues(t, 3.14, intAgg.Gauges["gauge0"])
	assert.EqualValues(t, 5.5, intAgg.Gauges["gauge1"]) // 5.5 takes precedence over 3.14 (argument to merge takes precedence.)

	// Spot-check timings:
	assert.EqualValues(t, 2, intAgg.Timers["timing0"].Count)
	assert.EqualValues(t, 24, intAgg.Timers["timing0"].NanosSum)

	// Spot-check event-errs:
	assert.EqualValues(t, 2, intAgg.EventErrs["err0"].Count)
	assert.EqualValues(t, []error{fmt.Errorf("wat")}, intAgg.EventErrs["err0"].getErrorSamples())

	// Spot-check jobs:
	job := intAgg.Jobs["job0"]
	assert.EqualValues(t, 2, job.CountSuccess)
	assert.EqualValues(t, 0, job.CountError)
	assert.EqualValues(t, 2, job.Events["event0"])
	assert.EqualValues(t, 0, job.Events["event4"])
	assert.EqualValues(t, 3.14, job.Gauges["gauge0"])
	assert.EqualValues(t, 2, job.Timers["timing0"].Count)
	assert.EqualValues(t, 24, job.Timers["timing0"].NanosSum)
	assert.EqualValues(t, 2, job.EventErrs["err0"].Count)
	assert.Equal(t, []error{fmt.Errorf("wat")}, job.EventErrs["err0"].getErrorSamples())
}
