package health

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

type testSink struct {
	LastErr *testEmitting
}

type testEmitting struct {
	Emitted    bool
	WasUnmuted bool
	WasMuted   bool
	WasRaw     bool
}

func (s *testSink) EmitEvent(job string, event string, kvs map[string]string) {}

func (s *testSink) EmitEventErr(job string, event string, inputErr error, kvs map[string]string) {
	var last testEmitting
	s.LastErr = &last
	switch inputErr := inputErr.(type) {
	case *UnmutedError:
		last.WasUnmuted = true
		last.Emitted = inputErr.Emitted
	case *MutedError:
		last.WasMuted = true
	default: // eg, case error:
		last.WasRaw = true
	}
}
func (s *testSink) EmitTiming(job string, event string, nanos int64, kvs map[string]string) {}
func (s *testSink) EmitComplete(job string, status CompletionStatus, nanos int64, kvs map[string]string) {
}

func TestUnmutedErrors(t *testing.T) {
	stream := NewStream()
	sink := &testSink{}
	stream.AddSink(sink)
	job := stream.NewJob("myjob")

	origErr := fmt.Errorf("wat")
	retErr := job.EventErr("abcd", origErr)

	// retErr is an UnmutedError with Emitted=true
	if retErr, ok := retErr.(*UnmutedError); ok {
		assert.True(t, retErr.Emitted)
		assert.Equal(t, retErr.Err, origErr)
	} else {
		t.Errorf("expected retErr to be an *UnmutedError")
	}

	// LastErr has Emitted=false, WasUnmuted=true
	assert.NotNil(t, sink.LastErr)
	assert.True(t, sink.LastErr.WasUnmuted)
	assert.False(t, sink.LastErr.Emitted)

	// Log it again!
	retErr2 := job.EventErr("abcdefg", retErr)

	// retErr is an UnmutedError with Emitted=true
	if retErr2, ok := retErr2.(*UnmutedError); ok {
		assert.True(t, retErr2.Emitted)
		assert.Equal(t, retErr2.Err, origErr) // We don't endlessly wrap UnmutedErrors inside UnmutedErrors
	} else {
		t.Errorf("expected retErr to be an *UnmutedError")
	}

	// LastErr has Emitted=false, WasUnmuted=true
	assert.NotNil(t, sink.LastErr)
	assert.True(t, sink.LastErr.WasUnmuted)
	assert.True(t, sink.LastErr.Emitted)
}

func TestMutedErrors(t *testing.T) {
	stream := NewStream()
	sink := &testSink{}
	stream.AddSink(sink)
	job := stream.NewJob("myjob")

	origErr := fmt.Errorf("wat")
	mutedOrig := Mute(origErr)
	retErr := job.EventErr("abcd", mutedOrig)

	// retErr is an UnmutedError with Emitted=true
	if retErr, ok := retErr.(*MutedError); ok {
		assert.Equal(t, retErr.Err, origErr)
	} else {
		t.Errorf("expected retErr to be an *MutedError")
	}

	// LastErr has Emitted=false, WasUnmuted=true
	assert.NotNil(t, sink.LastErr)
	assert.True(t, sink.LastErr.WasMuted)

	// Log it again!
	retErr2 := job.EventErr("abcdefg", retErr)

	// retErr is an UnmutedError with Emitted=true
	if retErr2, ok := retErr2.(*MutedError); ok {
		assert.Equal(t, retErr2.Err, origErr) // We don't endlessly wrap MutedErrors inside MutedErrors
	} else {
		t.Errorf("expected retErr to be an *MutedError")
	}

	// LastErr has Emitted=false, WasUnmuted=true
	assert.NotNil(t, sink.LastErr)
	assert.True(t, sink.LastErr.WasMuted)
}
