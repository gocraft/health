package health

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewAggregator(t *testing.T) {
	a := newAggregator(time.Minute, time.Minute*5)
	assert.Equal(t, time.Minute, a.intervalDuration)
	assert.Equal(t, time.Minute*5, a.retain)
	assert.Equal(t, 5, a.maxIntervals)
	assert.Equal(t, 0, len(a.intervalAggregations))
	assert.NotNil(t, a.intervalAggregations)
}

func TestEmitEvent(t *testing.T) {
	// Set time, and do a single event
	setNowMock("2011-09-09T23:36:13Z")
	defer resetNowMock()
	a := newAggregator(time.Minute, time.Minute*5)
	a.EmitEvent("foo", "bar")

	assert.Equal(t, 1, len(a.intervalAggregations))

	intAgg := a.intervalAggregations[0]
	assert.NotNil(t, intAgg.Events)
	assert.EqualValues(t, 1, intAgg.Events["bar"])
	assert.EqualValues(t, 1, intAgg.SerialNumber)

	assert.NotNil(t, intAgg.Jobs)
	jobAgg := intAgg.Jobs["foo"]
	assert.NotNil(t, jobAgg)
	assert.NotNil(t, jobAgg.Events)
	assert.EqualValues(t, 1, jobAgg.Events["bar"])

	// Now, without changing the time, we'll do 3 more events:
	a.EmitEvent("foo", "bar") // duplicate to above
	a.EmitEvent("foo", "baz") // same job, diff event
	a.EmitEvent("wat", "bar") // diff job, same event

	assert.Equal(t, 1, len(a.intervalAggregations))

	intAgg = a.intervalAggregations[0]
	assert.EqualValues(t, 3, intAgg.Events["bar"])
	assert.EqualValues(t, 4, intAgg.SerialNumber)

	jobAgg = intAgg.Jobs["foo"]
	assert.EqualValues(t, 2, jobAgg.Events["bar"])
	assert.EqualValues(t, 1, jobAgg.Events["baz"])

	jobAgg = intAgg.Jobs["wat"]
	assert.NotNil(t, jobAgg)
	assert.EqualValues(t, 1, jobAgg.Events["bar"])

	// Now we'll increment time and do one more event:
	setNowMock("2011-09-09T23:37:01Z")
	a.EmitEvent("foo", "bar")

	assert.Equal(t, 2, len(a.intervalAggregations))

	// make sure old values don't change:
	intAgg = a.intervalAggregations[0]
	assert.EqualValues(t, 3, intAgg.Events["bar"])
	assert.EqualValues(t, 4, intAgg.SerialNumber)

	intAgg = a.intervalAggregations[1]
	assert.EqualValues(t, 1, intAgg.Events["bar"])
	assert.EqualValues(t, 1, intAgg.SerialNumber)
}

func TestEmitEventErr(t *testing.T) {
	setNowMock("2011-09-09T23:36:13Z")
	defer resetNowMock()
	a := newAggregator(time.Minute, time.Minute*5)
	a.EmitEventErr("foo", "bar", errors.New("wat"))

	assert.Equal(t, 1, len(a.intervalAggregations))

	intAgg := a.intervalAggregations[0]
	assert.NotNil(t, intAgg.EventErrs)
	ce := intAgg.EventErrs["bar"]
	assert.NotNil(t, ce)
	assert.EqualValues(t, 1, ce.Count)
	assert.Equal(t, []error{errors.New("wat")}, ce.getErrorSamples())
	assert.EqualValues(t, 1, intAgg.SerialNumber)

	assert.NotNil(t, intAgg.Jobs)
	jobAgg := intAgg.Jobs["foo"]
	assert.NotNil(t, jobAgg)
	assert.NotNil(t, jobAgg.EventErrs)
	ce = jobAgg.EventErrs["bar"]
	assert.EqualValues(t, 1, ce.Count)
	assert.Equal(t, []error{errors.New("wat")}, ce.getErrorSamples())

	// One more event with the same error:
	a.EmitEventErr("foo", "bar", errors.New("wat"))

	intAgg = a.intervalAggregations[0]
	ce = intAgg.EventErrs["bar"]
	assert.EqualValues(t, 2, ce.Count)
	assert.Equal(t, []error{errors.New("wat")}, ce.getErrorSamples()) // doesn't change

	// One more event with diff error:
	a.EmitEventErr("foo", "bar", errors.New("lol"))

	intAgg = a.intervalAggregations[0]
	ce = intAgg.EventErrs["bar"]
	assert.EqualValues(t, 3, ce.Count)
	assert.Equal(t, []error{errors.New("wat"), errors.New("lol")}, ce.getErrorSamples()) // new error added
}

func TestEmitTiming(t *testing.T) {
	setNowMock("2011-09-09T23:36:13Z")
	defer resetNowMock()
	a := newAggregator(time.Minute, time.Minute*5)
	a.EmitTiming("foo", "bar", 100)

	assert.Equal(t, 1, len(a.intervalAggregations))

	intAgg := a.intervalAggregations[0]
	assert.NotNil(t, intAgg.Timers)
	assert.EqualValues(t, 1, intAgg.SerialNumber)
	tAgg := intAgg.Timers["bar"]
	assert.NotNil(t, tAgg)
	assert.EqualValues(t, 1, tAgg.Count)
	assert.EqualValues(t, 100, tAgg.NanosSum)
	assert.EqualValues(t, 10000, tAgg.NanosSumSquares)
	assert.EqualValues(t, 100, tAgg.NanosMin)
	assert.EqualValues(t, 100, tAgg.NanosMax)

	assert.NotNil(t, intAgg.Jobs)
	jobAgg := intAgg.Jobs["foo"]
	assert.NotNil(t, jobAgg)
	assert.NotNil(t, jobAgg.Timers)
	tAgg = jobAgg.Timers["bar"]
	assert.EqualValues(t, 1, tAgg.Count)
	assert.EqualValues(t, 100, tAgg.NanosSum)
	assert.EqualValues(t, 10000, tAgg.NanosSumSquares)
	assert.EqualValues(t, 100, tAgg.NanosMin)
	assert.EqualValues(t, 100, tAgg.NanosMax)

	// Another timing:
	a.EmitTiming("baz", "bar", 9) // note: diff job

	intAgg = a.intervalAggregations[0]
	tAgg = intAgg.Timers["bar"]
	assert.NotNil(t, tAgg)
	assert.EqualValues(t, 2, tAgg.Count)
	assert.EqualValues(t, 109, tAgg.NanosSum)
	assert.EqualValues(t, 10081, tAgg.NanosSumSquares)
	assert.EqualValues(t, 9, tAgg.NanosMin)
	assert.EqualValues(t, 100, tAgg.NanosMax)

	jobAgg = intAgg.Jobs["baz"]
	tAgg = jobAgg.Timers["bar"]
	assert.EqualValues(t, 1, tAgg.Count)
	assert.EqualValues(t, 9, tAgg.NanosSum)
	assert.EqualValues(t, 81, tAgg.NanosSumSquares)
	assert.EqualValues(t, 9, tAgg.NanosMin)
	assert.EqualValues(t, 9, tAgg.NanosMax)
}

func TestEmitGauge(t *testing.T) {
	setNowMock("2011-09-09T23:36:13Z")
	defer resetNowMock()
	a := newAggregator(time.Minute, time.Minute*5)
	a.EmitGauge("foo", "bar", 100)

	assert.Equal(t, 1, len(a.intervalAggregations))

	intAgg := a.intervalAggregations[0]
	assert.NotNil(t, intAgg.Gauges)
	assert.EqualValues(t, 1, intAgg.SerialNumber)
	v, ok := intAgg.Gauges["bar"]
	assert.True(t, ok)
	assert.Equal(t, 100.0, v)

	assert.NotNil(t, intAgg.Jobs)
	jobAgg := intAgg.Jobs["foo"]
	assert.NotNil(t, jobAgg)
	assert.NotNil(t, jobAgg.Gauges)
	v, ok = intAgg.Gauges["bar"]
	assert.True(t, ok)
	assert.Equal(t, 100.0, v)

	// Another gauge:
	a.EmitGauge("baz", "bar", 3.14) // note: diff job

	intAgg = a.intervalAggregations[0]
	v, ok = intAgg.Gauges["bar"]
	assert.True(t, ok)
	assert.Equal(t, 3.14, v)

	jobAgg = intAgg.Jobs["baz"]
	v, ok = intAgg.Gauges["bar"]
	assert.True(t, ok)
	assert.Equal(t, 3.14, v)
}

func TestEmitComplete(t *testing.T) {
	setNowMock("2011-09-09T23:36:13Z")
	defer resetNowMock()
	a := newAggregator(time.Minute, time.Minute*5)
	a.EmitComplete("foo", Success, 100)
	a.EmitComplete("foo", ValidationError, 5)
	a.EmitComplete("foo", Panic, 9)
	a.EmitComplete("foo", Error, 7)
	a.EmitComplete("foo", Junk, 11)

	assert.Equal(t, 1, len(a.intervalAggregations))

	intAgg := a.intervalAggregations[0]
	assert.EqualValues(t, 5, intAgg.SerialNumber)
	jobAgg := intAgg.Jobs["foo"]
	assert.NotNil(t, jobAgg)

	assert.EqualValues(t, 5, jobAgg.Count)
	assert.EqualValues(t, 1, jobAgg.CountSuccess)
	assert.EqualValues(t, 1, jobAgg.CountValidationError)
	assert.EqualValues(t, 1, jobAgg.CountPanic)
	assert.EqualValues(t, 1, jobAgg.CountError)
	assert.EqualValues(t, 1, jobAgg.CountJunk)
	assert.EqualValues(t, 132, jobAgg.NanosSum)
	assert.EqualValues(t, 10276, jobAgg.NanosSumSquares)
	assert.EqualValues(t, 5, jobAgg.NanosMin)
	assert.EqualValues(t, 100, jobAgg.NanosMax)
}

func TestRotation(t *testing.T) {
	defer resetNowMock()
	a := newAggregator(time.Minute, time.Minute*5)
	setNowMock("2011-09-09T23:36:13Z")
	a.EmitEvent("foo", "bar")

	setNowMock("2011-09-09T23:37:13Z")
	a.EmitEvent("foo", "bar")
	a.EmitEvent("foo", "bar")

	setNowMock("2011-09-09T23:38:13Z")
	a.EmitEvent("foo", "bar")
	a.EmitEvent("foo", "bar")
	a.EmitEvent("foo", "bar")

	setNowMock("2011-09-09T23:39:13Z")
	a.EmitEvent("foo", "bar")
	a.EmitEvent("foo", "bar")
	a.EmitEvent("foo", "bar")
	a.EmitEvent("foo", "bar")

	setNowMock("2011-09-09T23:40:13Z")
	a.EmitEvent("foo", "bar")
	a.EmitEvent("foo", "bar")
	a.EmitEvent("foo", "bar")
	a.EmitEvent("foo", "bar")
	a.EmitEvent("foo", "bar")

	assert.Equal(t, 5, len(a.intervalAggregations))

	for i := 0; i < 5; i++ {
		intAgg := a.intervalAggregations[i]
		assert.EqualValues(t, i+1, intAgg.Events["bar"])
	}

	setNowMock("2011-09-09T23:41:13Z")
	a.EmitEvent("foo", "ok")

	assert.Equal(t, 5, len(a.intervalAggregations))

	for i := 0; i < 4; i++ {
		intAgg := a.intervalAggregations[i]
		assert.EqualValues(t, i+2, intAgg.Events["bar"])
	}
	intAgg := a.intervalAggregations[4]
	assert.EqualValues(t, 0, intAgg.Events["bar"])
	assert.EqualValues(t, 1, intAgg.Events["ok"])

}
