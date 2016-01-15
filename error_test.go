package health

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

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
	assert.True(t, sink.LastErrUnmuted)
	assert.False(t, sink.LastErrEmitted)

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
	assert.True(t, sink.LastErrUnmuted)
	assert.True(t, sink.LastErrEmitted)
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
	assert.True(t, sink.LastErrMuted)

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
	assert.True(t, sink.LastErrMuted)
}
