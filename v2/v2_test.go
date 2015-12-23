package v2

import (
	"errors"
	"fmt"
	"github.com/gocraft/health"
	"github.com/stretchr/testify/assert"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"
)

var testAddr = "127.0.0.1:7890"

////
var testErr = errors.New("my test error")

var completionStatusToString = map[health.CompletionStatus]string{
	health.Success:         "success",
	health.ValidationError: "validation_error",
	health.Panic:           "panic",
	health.Error:           "error",
	health.Junk:            "junk",
}

////

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
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"metroid.my.event:1|c\nmetroid.my.job.my.event:1|c\n"}, func() {
		sink.EmitEvent("my.job", "my.event", nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitEventShouldSanitize(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, "metroid")
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"metroid.my$event:1|c\nmetroid.my$job.my$event:1|c\n"}, func() {
		sink.EmitEvent("my|job", "my:event", nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitEventNoPrefix(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, "")
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"my.event:1|c\nmy.job.my.event:1|c\n"}, func() {
		sink.EmitEvent("my.job", "my.event", nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitEventErrPrefix(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, "metroid")
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"metroid.my.event.error:1|c\nmetroid.my.job.my.event.error:1|c\n"}, func() {
		sink.EmitEventErr("my.job", "my.event", testErr, nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitEventErrNoPrefix(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, "")
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"my.event.error:1|c\nmy.job.my.event.error:1|c\n"}, func() {
		sink.EmitEventErr("my.job", "my.event", testErr, nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitTimingPrefix(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, "metroid")
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"metroid.my.event:123|ms\nmetroid.my.job.my.event:123|ms\n"}, func() {
		sink.EmitTiming("my.job", "my.event", 123456789, nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitTimingNoPrefix(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, "")
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"my.event:123|ms\nmy.job.my.event:123|ms\n"}, func() {
		sink.EmitTiming("my.job", "my.event", 123456789, nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitTimingShort(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, "")
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"my.event:1.23|ms\nmy.job.my.event:1.23|ms\n"}, func() {
		sink.EmitTiming("my.job", "my.event", 1234567, nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitGaugePrefix(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, "metroid")
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"metroid.my.event:3.14|g\nmetroid.my.job.my.event:3.14|g\n"}, func() {
		sink.EmitGauge("my.job", "my.event", 3.14, nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitGaugeNoPrefix(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, "")
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"my.event:3.00|g\nmy.job.my.event:3.00|g\n"}, func() {
		sink.EmitGauge("my.job", "my.event", 3, nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitCompletePrefix(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, "metroid")
	defer sink.Stop()
	assert.NoError(t, err)
	for kind, kindStr := range completionStatusToString {
		str := fmt.Sprintf("metroid.my.job.%s:129|ms\n", kindStr)
		listenFor(t, []string{str}, func() {
			sink.EmitComplete("my.job", kind, 129456789, nil)
			sink.Drain()
		})
	}
}

func TestStatsDSinkEmitCompleteNoPrefix(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, "")
	defer sink.Stop()
	assert.NoError(t, err)
	for kind, kindStr := range completionStatusToString {
		str := fmt.Sprintf("my.job.%s:129|ms\n", kindStr)
		listenFor(t, []string{str}, func() {
			sink.EmitComplete("my.job", kind, 129456789, nil)
			sink.Drain()
		})
	}
}

func TestStatsDSinkEmitTimingSubMillisecond(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, "metroid")
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"metroid.my.event:0.46|ms\nmetroid.my.job.my.event:0.46|ms\n"}, func() {
		sink.EmitTiming("my.job", "my.event", 456789, nil)
		sink.Drain()
	})
}
