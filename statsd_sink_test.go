package health

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"net"
	"runtime"
	"strings"
	"sync"
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

	buf := make([]byte, 10000)
	for _, msg := range msgs {
		err = c.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
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

func TestStatsDSinkPeriodicPurge(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, &StatsDSinkOptions{Prefix: "metroid"})
	assert.NoError(t, err)

	// Stop the sink, set a smaller flush period, and start it agian
	sink.Stop()
	sink.flushPeriod = 1 * time.Millisecond
	go sink.loop()
	defer sink.Stop()

	listenFor(t, []string{"metroid.my.event:1|c\nmetroid.my.job.my.event:1|c\n"}, func() {
		sink.EmitEvent("my.job", "my.event", nil)
		time.Sleep(10 * time.Millisecond)
	})
}

func TestStatsDSinkPacketLimit(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, &StatsDSinkOptions{Prefix: "metroid", SkipNestedEvents: true})
	assert.NoError(t, err)

	// s is 101 bytes
	s := "metroid." + strings.Repeat("a", 88) + ":1|c\n"

	// expect 1 packet that is 14*101=1414 bytes, and the next one to be 101 bytes
	listenFor(t, []string{strings.Repeat(s, 14), s}, func() {
		for i := 0; i < 15; i++ {
			sink.EmitEvent("my.job", strings.Repeat("a", 88), nil)
		}

		sink.Drain()
	})
}

func TestStatsDSinkEmitEventPrefix(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, &StatsDSinkOptions{Prefix: "metroid"})
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"metroid.my.event:1|c\nmetroid.my.job.my.event:1|c\n"}, func() {
		sink.EmitEvent("my.job", "my.event", nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitEventShouldSanitize(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, &StatsDSinkOptions{Prefix: "metroid"})
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"metroid.my$event:1|c\nmetroid.my$job.my$event:1|c\n"}, func() {
		sink.EmitEvent("my|job", "my:event", nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitEventNoPrefix(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, nil)
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"my.event:1|c\nmy.job.my.event:1|c\n"}, func() {
		sink.EmitEvent("my.job", "my.event", nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitEventSkipNested(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, &StatsDSinkOptions{SkipNestedEvents: true})
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"my.event:1|c\n"}, func() {
		sink.EmitEvent("my.job", "my.event", nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitEventSkipTopLevel(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, &StatsDSinkOptions{SkipTopLevelEvents: true})
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"my.job.my.event:1|c\n"}, func() {
		sink.EmitEvent("my.job", "my.event", nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitEventErrPrefix(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, &StatsDSinkOptions{Prefix: "metroid"})
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"metroid.my.event.error:1|c\nmetroid.my.job.my.event.error:1|c\n"}, func() {
		sink.EmitEventErr("my.job", "my.event", testErr, nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitEventErrNoPrefix(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, nil)
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"my.event.error:1|c\nmy.job.my.event.error:1|c\n"}, func() {
		sink.EmitEventErr("my.job", "my.event", testErr, nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitEventErrSkipNested(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, &StatsDSinkOptions{SkipNestedEvents: true})
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"my.event.error:1|c\n"}, func() {
		sink.EmitEventErr("my.job", "my.event", testErr, nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitEventErrSkipTopLevel(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, &StatsDSinkOptions{SkipTopLevelEvents: true})
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"my.job.my.event.error:1|c\n"}, func() {
		sink.EmitEventErr("my.job", "my.event", testErr, nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitTimingPrefix(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, &StatsDSinkOptions{Prefix: "metroid"})
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"metroid.my.event:123|ms\nmetroid.my.job.my.event:123|ms\n"}, func() {
		sink.EmitTiming("my.job", "my.event", 123456789, nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitTimingNoPrefix(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, nil)
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"my.event:123|ms\nmy.job.my.event:123|ms\n"}, func() {
		sink.EmitTiming("my.job", "my.event", 123456789, nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitTimingSkipNested(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, &StatsDSinkOptions{SkipNestedEvents: true})
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"my.event:123|ms\n"}, func() {
		sink.EmitTiming("my.job", "my.event", 123456789, nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitTimingSkipTopLevel(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, &StatsDSinkOptions{SkipTopLevelEvents: true})
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"my.job.my.event:123|ms\n"}, func() {
		sink.EmitTiming("my.job", "my.event", 123456789, nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitTimingShort(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, nil)
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"my.event:1.23|ms\nmy.job.my.event:1.23|ms\n"}, func() {
		sink.EmitTiming("my.job", "my.event", 1234567, nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitGaugePrefix(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, &StatsDSinkOptions{Prefix: "metroid"})
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"metroid.my.event:3.14|g\nmetroid.my.job.my.event:3.14|g\n"}, func() {
		sink.EmitGauge("my.job", "my.event", 3.14, nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitGaugeSmall(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, &StatsDSinkOptions{Prefix: "metroid", SkipNestedEvents: true})
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"metroid.my.event:0.14|g\nmetroid.my.event:0.0401|g\nmetroid.my.event:-0.0001|g\n"}, func() {
		sink.EmitGauge("my.job", "my.event", 0.1401, nil)
		sink.EmitGauge("my.job", "my.event", 0.0401, nil)
		sink.EmitGauge("my.job", "my.event", -0.0001, nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitGaugeNoPrefix(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, nil)
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"my.event:3.00|g\nmy.job.my.event:3.00|g\n"}, func() {
		sink.EmitGauge("my.job", "my.event", 3, nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitGaugeSkipNested(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, &StatsDSinkOptions{SkipNestedEvents: true})
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"my.event:3.00|g\n"}, func() {
		sink.EmitGauge("my.job", "my.event", 3, nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitGaugeSkipTopLevel(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, &StatsDSinkOptions{SkipTopLevelEvents: true})
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"my.job.my.event:3.00|g\n"}, func() {
		sink.EmitGauge("my.job", "my.event", 3, nil)
		sink.Drain()
	})
}

func TestStatsDSinkEmitCompletePrefix(t *testing.T) {
	sink, err := NewStatsDSink(testAddr, &StatsDSinkOptions{Prefix: "metroid"})
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
	sink, err := NewStatsDSink(testAddr, nil)
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
	sink, err := NewStatsDSink(testAddr, &StatsDSinkOptions{Prefix: "metroid"})
	defer sink.Stop()
	assert.NoError(t, err)
	listenFor(t, []string{"metroid.my.event:0.46|ms\nmetroid.my.job.my.event:0.46|ms\n"}, func() {
		sink.EmitTiming("my.job", "my.event", 456789, nil)
		sink.Drain()
	})
}

func BenchmarkStatsDSinkProcessEvent(b *testing.B) {
	sink, _ := NewStatsDSink(testAddr, &StatsDSinkOptions{Prefix: "metroid"})
	sink.Stop() // Don't do periodic things while we're benching

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sink.processEvent("myjob", "myevent")
	}
}

func BenchmarkStatsDSinkProcessEventErr(b *testing.B) {
	sink, _ := NewStatsDSink(testAddr, &StatsDSinkOptions{Prefix: "metroid"})
	sink.Stop() // Don't do periodic things while we're benching

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sink.processEventErr("myjob", "myevent")
	}
}

func BenchmarkStatsDSinkProcessTimingBig(b *testing.B) {
	sink, _ := NewStatsDSink(testAddr, &StatsDSinkOptions{Prefix: "metroid"})
	sink.Stop() // Don't do periodic things while we're benching

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sink.processTiming("myjob", "myevent", 30000000)
	}
}

func BenchmarkStatsDSinkProcessTimingSmall(b *testing.B) {
	sink, _ := NewStatsDSink(testAddr, &StatsDSinkOptions{Prefix: "metroid"})
	sink.Stop() // Don't do periodic things while we're benching

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sink.processTiming("myjob", "myevent", 1230000)
	}
}

func BenchmarkStatsDSinkProcessGauge(b *testing.B) {
	sink, _ := NewStatsDSink(testAddr, &StatsDSinkOptions{Prefix: "metroid"})
	sink.Stop() // Don't do periodic things while we're benching

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sink.processGauge("myjob", "myevent", 3.14)
	}
}

func BenchmarkStatsDSinkProcessComplete(b *testing.B) {
	sink, _ := NewStatsDSink(testAddr, &StatsDSinkOptions{Prefix: "metroid"})
	sink.Stop() // Don't do periodic things while we're benching

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sink.processComplete("myjob", Success, 1230000)
	}
}

func BenchmarkStatsDSinkOverall(b *testing.B) {
	const numGoroutines = 100
	var requestsPerGoroutine = b.N / numGoroutines

	stream := NewStream()
	sink, _ := NewStatsDSink(testAddr, &StatsDSinkOptions{Prefix: "metroid"})
	stream.AddSink(sink)
	job := stream.NewJob("foo")

	wg := sync.WaitGroup{}
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			for j := 0; j < requestsPerGoroutine; j++ {
				job.Event("evt")
			}
			wg.Done()
		}()
	}

	wg.Wait()
	sink.Drain()
}
