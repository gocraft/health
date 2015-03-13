package health

import (
	// "bytes"
	// "errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"
)

var testAddr = "127.0.0.1:7890"

func callerInfo() string {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		return ""
	}
	parts := strings.Split(file, "/")
	file = parts[len(parts)-1]
	return fmt.Sprintf("%s:%d", file, line)
}

func listenFor(t *testing.T, msgs []string, f func()) {
	c, err := net.ListenPacket("udp", testAddr)
	defer c.Close()
	assert.NoError(t, err)

	f()

	buf := make([]byte, 1024)
	for _, msg := range msgs {
		err = c.SetReadDeadline(time.Now().Add(1 * time.Second))
		assert.NoError(t, err)
		nbytes, _, err := c.ReadFrom(buf)
		assert.NoError(t, err)
		if err == nil {
			gotMsg := string(buf[0:nbytes])
			if gotMsg != msg {
				t.Errorf("Expected UPD packet %s but got %s\n", msg, gotMsg)
			}
		}
	}
}

func TestStatsDSinkEmitEventPrefix(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, "metroid")
	assert.NoError(t, err)
	listenFor(t, []string{"metroid.my.event:1|c\n", "metroid.my.job.my.event:1|c\n"}, func() {
		sink.EmitEvent("my.job", "my.event", nil)
	})
}

func TestStatsDSinkEmitEventShouldSanitize(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, "metroid")
	assert.NoError(t, err)
	listenFor(t, []string{"metroid.my$event:1|c\n", "metroid.my$job.my$event:1|c\n"}, func() {
		sink.EmitEvent("my|job", "my:event", nil)
	})
}

func TestStatsDSinkEmitEventNoPrefix(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, "")
	assert.NoError(t, err)
	listenFor(t, []string{"my.event:1|c\n", "my.job.my.event:1|c\n"}, func() {
		sink.EmitEvent("my.job", "my.event", nil)
	})
}

func TestStatsDSinkEmitEventErrPrefix(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, "metroid")
	assert.NoError(t, err)
	listenFor(t, []string{"metroid.my.event.error:1|c\n", "metroid.my.job.my.event.error:1|c\n"}, func() {
		sink.EmitEventErr("my.job", "my.event", testErr, nil)
	})
}

func TestStatsDSinkEmitEventErrNoPrefix(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, "")
	assert.NoError(t, err)
	listenFor(t, []string{"my.event.error:1|c\n", "my.job.my.event.error:1|c\n"}, func() {
		sink.EmitEventErr("my.job", "my.event", testErr, nil)
	})
}

func TestStatsDSinkEmitTimingPrefix(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, "metroid")
	assert.NoError(t, err)
	listenFor(t, []string{"metroid.my.event:123.456789|ms\n", "metroid.my.job.my.event:123.456789|ms\n"}, func() {
		sink.EmitTiming("my.job", "my.event", 123456789, nil)
	})
}

func TestStatsDSinkEmitTimingNoPrefix(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, "")
	assert.NoError(t, err)
	listenFor(t, []string{"my.event:123.456789|ms\n", "my.job.my.event:123.456789|ms\n"}, func() {
		sink.EmitTiming("my.job", "my.event", 123456789, nil)
	})
}

func TestStatsDSinkEmitCompletePrefix(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, "metroid")
	assert.NoError(t, err)
	for kind, kindStr := range completionStatusToString {
		str := fmt.Sprintf("metroid.my.job.%s:129.456789|ms\n", kindStr)
		listenFor(t, []string{str}, func() {
			sink.EmitComplete("my.job", kind, 129456789, nil)
		})
	}
}

func TestStatsDSinkEmitCompleteNoPrefix(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, "")
	assert.NoError(t, err)
	for kind, kindStr := range completionStatusToString {
		str := fmt.Sprintf("my.job.%s:129.456789|ms\n", kindStr)
		listenFor(t, []string{str}, func() {
			sink.EmitComplete("my.job", kind, 129456789, nil)
		})
	}
}

func TestStatsDSinkEmitTimingSubMillisecond(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, "metroid")
	assert.NoError(t, err)
	listenFor(t, []string{"metroid.my.event:0.456789|ms\n", "metroid.my.job.my.event:0.456789|ms\n"}, func() {
		sink.EmitTiming("my.job", "my.event", 456789, nil)
	})
}
