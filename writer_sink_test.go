package health

import (
	"bytes"
	"errors"
	"github.com/stretchr/testify/assert"
	"regexp"
	"testing"
)

var basicEventRegexp = regexp.MustCompile("\\[[^\\]]+\\]: job:(.+) event:(.+)")
var kvsEventRegexp = regexp.MustCompile("\\[[^\\]]+\\]: job:(.+) event:(.+) kvs:\\[(.+)\\]")
var basicEventErrRegexp = regexp.MustCompile("\\[[^\\]]+\\]: job:(.+) event:(.+) err:(.+)")
var kvsEventErrRegexp = regexp.MustCompile("\\[[^\\]]+\\]: job:(.+) event:(.+) err:(.+) kvs:\\[(.+)\\]")
var basicTimingRegexp = regexp.MustCompile("\\[[^\\]]+\\]: job:(.+) event:(.+) time:(.+)")
var kvsTimingRegexp = regexp.MustCompile("\\[[^\\]]+\\]: job:(.+) event:(.+) time:(.+) kvs:\\[(.+)\\]")
var basicGaugeRegexp = regexp.MustCompile("\\[[^\\]]+\\]: job:(.+) event:(.+) gauge:(.+)")
var kvsGaugeRegexp = regexp.MustCompile("\\[[^\\]]+\\]: job:(.+) event:(.+) gauge:(.+) kvs:\\[(.+)\\]")
var basicCompletionRegexp = regexp.MustCompile("\\[[^\\]]+\\]: job:(.+) status:(.+) time:(.+)")
var kvsCompletionRegexp = regexp.MustCompile("\\[[^\\]]+\\]: job:(.+) status:(.+) time:(.+) kvs:\\[(.+)\\]")

var testErr = errors.New("my test error")

func BenchmarkWriterSinkEmitEvent(b *testing.B) {
	var by bytes.Buffer
	someKvs := map[string]string{"foo": "bar", "qux": "dog"}
	sink := WriterSink{&by}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		by.Reset()
		sink.EmitEvent("myjob", "myevent", someKvs)
	}
}

func BenchmarkWriterSinkEmitEventErr(b *testing.B) {
	var by bytes.Buffer
	someKvs := map[string]string{"foo": "bar", "qux": "dog"}
	sink := WriterSink{&by}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		by.Reset()
		sink.EmitEventErr("myjob", "myevent", testErr, someKvs)
	}
}

func BenchmarkWriterSinkEmitTiming(b *testing.B) {
	var by bytes.Buffer
	someKvs := map[string]string{"foo": "bar", "qux": "dog"}
	sink := WriterSink{&by}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		by.Reset()
		sink.EmitTiming("myjob", "myevent", 234203, someKvs)
	}
}

func BenchmarkWriterSinkEmitComplete(b *testing.B) {
	var by bytes.Buffer
	someKvs := map[string]string{"foo": "bar", "qux": "dog"}
	sink := WriterSink{&by}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		by.Reset()
		sink.EmitComplete("myjob", Success, 234203, someKvs)
	}
}

func TestWriterSinkEmitEventBasic(t *testing.T) {
	var b bytes.Buffer
	sink := WriterSink{&b}
	sink.EmitEvent("myjob", "myevent", nil)

	str := b.String()

	result := basicEventRegexp.FindStringSubmatch(str)
	assert.Equal(t, 3, len(result))
	assert.Equal(t, "myjob", result[1])
	assert.Equal(t, "myevent", result[2])
}

func TestWriterSinkEmitEventKvs(t *testing.T) {
	var b bytes.Buffer
	sink := WriterSink{&b}
	sink.EmitEvent("myjob", "myevent", map[string]string{"wat": "ok", "another": "thing"})

	str := b.String()

	result := kvsEventRegexp.FindStringSubmatch(str)
	assert.Equal(t, 4, len(result))
	assert.Equal(t, "myjob", result[1])
	assert.Equal(t, "myevent", result[2])
	assert.Equal(t, "another:thing wat:ok", result[3])
}

func TestWriterSinkEmitEventErrBasic(t *testing.T) {
	var b bytes.Buffer
	sink := WriterSink{&b}
	sink.EmitEventErr("myjob", "myevent", testErr, nil)

	str := b.String()

	result := basicEventErrRegexp.FindStringSubmatch(str)
	assert.Equal(t, 4, len(result))
	assert.Equal(t, "myjob", result[1])
	assert.Equal(t, "myevent", result[2])
	assert.Equal(t, testErr.Error(), result[3])
}

func TestWriterSinkEmitEventErrKvs(t *testing.T) {
	var b bytes.Buffer
	sink := WriterSink{&b}
	sink.EmitEventErr("myjob", "myevent", testErr, map[string]string{"wat": "ok", "another": "thing"})

	str := b.String()

	result := kvsEventErrRegexp.FindStringSubmatch(str)
	assert.Equal(t, 5, len(result))
	assert.Equal(t, "myjob", result[1])
	assert.Equal(t, "myevent", result[2])
	assert.Equal(t, testErr.Error(), result[3])
	assert.Equal(t, "another:thing wat:ok", result[4])
}

func TestWriterSinkEmitTimingBasic(t *testing.T) {
	var b bytes.Buffer
	sink := WriterSink{&b}
	sink.EmitTiming("myjob", "myevent", 1204000, nil)

	str := b.String()

	result := basicTimingRegexp.FindStringSubmatch(str)
	assert.Equal(t, 4, len(result))
	assert.Equal(t, "myjob", result[1])
	assert.Equal(t, "myevent", result[2])
	assert.Equal(t, "1204 μs", result[3])
}

func TestWriterSinkEmitTimingKvs(t *testing.T) {
	var b bytes.Buffer
	sink := WriterSink{&b}
	sink.EmitTiming("myjob", "myevent", 34567890, map[string]string{"wat": "ok", "another": "thing"})

	str := b.String()

	result := kvsTimingRegexp.FindStringSubmatch(str)
	assert.Equal(t, 5, len(result))
	assert.Equal(t, "myjob", result[1])
	assert.Equal(t, "myevent", result[2])
	assert.Equal(t, "34 ms", result[3])
	assert.Equal(t, "another:thing wat:ok", result[4])
}

func TestWriterSinkEmitGaugeBasic(t *testing.T) {
	var b bytes.Buffer
	sink := WriterSink{&b}
	sink.EmitGauge("myjob", "myevent", 3.14, nil)

	str := b.String()

	result := basicGaugeRegexp.FindStringSubmatch(str)
	assert.Equal(t, 4, len(result))
	assert.Equal(t, "myjob", result[1])
	assert.Equal(t, "myevent", result[2])
	assert.Equal(t, "3.14", result[3])
}

func TestWriterSinkEmitGaugeKvs(t *testing.T) {
	var b bytes.Buffer
	sink := WriterSink{&b}
	sink.EmitGauge("myjob", "myevent", 0.11, map[string]string{"wat": "ok", "another": "thing"})

	str := b.String()

	result := kvsGaugeRegexp.FindStringSubmatch(str)
	assert.Equal(t, 5, len(result))
	assert.Equal(t, "myjob", result[1])
	assert.Equal(t, "myevent", result[2])
	assert.Equal(t, "0.11", result[3])
	assert.Equal(t, "another:thing wat:ok", result[4])
}

func TestWriterSinkEmitCompleteBasic(t *testing.T) {
	for kind, kindStr := range completionStatusToString {
		var b bytes.Buffer
		sink := WriterSink{&b}
		sink.EmitComplete("myjob", kind, 1204000, nil)

		str := b.String()

		result := basicCompletionRegexp.FindStringSubmatch(str)
		assert.Equal(t, 4, len(result))
		assert.Equal(t, "myjob", result[1])
		assert.Equal(t, kindStr, result[2])
		assert.Equal(t, "1204 μs", result[3])
	}
}

func TestWriterSinkEmitCompleteKvs(t *testing.T) {
	for kind, kindStr := range completionStatusToString {
		var b bytes.Buffer
		sink := WriterSink{&b}
		sink.EmitComplete("myjob", kind, 34567890, map[string]string{"wat": "ok", "another": "thing"})

		str := b.String()

		result := kvsCompletionRegexp.FindStringSubmatch(str)
		assert.Equal(t, 5, len(result))
		assert.Equal(t, "myjob", result[1])
		assert.Equal(t, kindStr, result[2])
		assert.Equal(t, "34 ms", result[3])
		assert.Equal(t, "another:thing wat:ok", result[4])
	}
}
