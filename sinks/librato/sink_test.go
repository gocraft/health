package librato

import (
	"fmt"
	"github.com/gocraft/health"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestNewShutdown(t *testing.T) {
	s := New("a", "b", "c")
	defer s.Stop()

	assert.Equal(t, "a", s.libratoUser)
	assert.Equal(t, "b", s.libratoApiKey)
	assert.Equal(t, "c", s.prefix)
}

func TestEmit(t *testing.T) {
	s := New("a", "b", "c")

	s.EmitEvent("cool", "story", nil)
	s.EmitEvent("cool", "story", nil)
	s.EmitEvent("cool", "story", nil)

	s.EmitEventErr("sad", "day", fmt.Errorf("ok"), nil)
	s.EmitEventErr("sad", "day", fmt.Errorf("ok"), nil)

	s.EmitTiming("rad", "dino", 6000000, nil)
	s.EmitTiming("bad", "dino", 12000000, nil)

	s.EmitComplete("tylersmith", health.Success, 22000000, nil)
	s.EmitComplete("tylersmart", health.Junk, 8000000, nil)

	time.Sleep(3 * time.Millisecond)
	s.Stop()

	assert.Equal(t, int64(3), s.counters["c.story.count"])
	assert.Equal(t, int64(3), s.counters["c.cool.story.count"])
	assert.Equal(t, int64(2), s.counters["c.day.error.count"])
	assert.Equal(t, int64(2), s.counters["c.sad.day.error.count"])

	g := s.timers["c.dino.timing"]
	assert.Equal(t, int64(2), g.Count)
	assert.Equal(t, 18.0, g.Sum)
	assert.Equal(t, 6.0, g.Min)
	assert.Equal(t, 12.0, g.Max)
	assert.Equal(t, 180.0, g.SumSquares)
	assert.Equal(t, defaultTimerAttributes, g.Attributes)

	g = s.timers["c.rad.dino.timing"]
	assert.Equal(t, int64(1), g.Count)
	assert.Equal(t, 6.0, g.Sum)
	assert.Equal(t, 6.0, g.Min)
	assert.Equal(t, 6.0, g.Max)
	assert.Equal(t, 36.0, g.SumSquares)

	g = s.timers["c.bad.dino.timing"]
	assert.Equal(t, int64(1), g.Count)
	assert.Equal(t, 12.0, g.Sum)
	assert.Equal(t, 12.0, g.Min)
	assert.Equal(t, 12.0, g.Max)
	assert.Equal(t, 144.0, g.SumSquares)

	g = s.timers["c.tylersmith.success.timing"]
	assert.Equal(t, int64(1), g.Count)
	assert.Equal(t, 22.0, g.Sum)
	assert.Equal(t, 22.0, g.Min)
	assert.Equal(t, 22.0, g.Max)
	assert.Equal(t, 484.0, g.SumSquares)

	g = s.timers["c.tylersmart.junk.timing"]
	assert.Equal(t, int64(1), g.Count)
	assert.Equal(t, 8.0, g.Sum)
	assert.Equal(t, 8.0, g.Min)
	assert.Equal(t, 8.0, g.Max)
	assert.Equal(t, 64.0, g.SumSquares)

}
