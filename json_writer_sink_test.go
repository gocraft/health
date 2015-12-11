package health

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testJsonEvent struct {
	Job         string
	Event       string
	Timestamp   string
	Err         string
	Nanoseconds int64
	Value       float64
	Status      string
	Kvs         map[string]string
}

func TestJsonWriterSinkEvent(t *testing.T) {
	var buf bytes.Buffer
	someKvs := map[string]string{"foo": "bar", "qux": "dog"}
	sink := JsonWriterSink{&buf}
	sink.EmitEvent("myjob", "myevent", someKvs)

	dec := json.NewDecoder(&buf)
	event := &testJsonEvent{}
	err := dec.Decode(event)

	assert.NoError(t, err)
	assert.Equal(t, "bar", event.Kvs["foo"])
	assert.Equal(t, "myjob", event.Job)
	assert.Equal(t, "myevent", event.Event)
}

func TestJsonWriterSinkEventErr(t *testing.T) {
	var buf bytes.Buffer
	sink := JsonWriterSink{&buf}
	someKvs := map[string]string{"foo": "bar", "qux": "dog"}
	sink.EmitEventErr("myjob", "myevent", errors.New("test err"), someKvs)

	dec := json.NewDecoder(&buf)
	event := &testJsonEvent{}
	err := dec.Decode(event)

	assert.NoError(t, err)
	assert.Equal(t, "bar", event.Kvs["foo"])
	assert.Equal(t, "myjob", event.Job)
	assert.Equal(t, "myevent", event.Event)
	assert.Equal(t, "test err", event.Err)
}

func TestJsonWriterSinkEventTiming(t *testing.T) {
	var buf bytes.Buffer
	sink := JsonWriterSink{&buf}
	someKvs := map[string]string{"foo": "bar", "qux": "dog"}
	sink.EmitTiming("myjob", "myevent", 34567890, someKvs)

	event := &testJsonEvent{}
	dec := json.NewDecoder(&buf)
	err := dec.Decode(event)

	assert.NoError(t, err)
	assert.Equal(t, "bar", event.Kvs["foo"])
	assert.Equal(t, "myjob", event.Job)
	assert.Equal(t, "myevent", event.Event)
	assert.EqualValues(t, 34567890, event.Nanoseconds)
}

func TestJsonWriterSinkEventGauge(t *testing.T) {
	var buf bytes.Buffer
	sink := JsonWriterSink{&buf}
	someKvs := map[string]string{"foo": "bar", "qux": "dog"}
	sink.EmitGauge("myjob", "myevent", 3.14, someKvs)

	event := &testJsonEvent{}
	dec := json.NewDecoder(&buf)
	err := dec.Decode(event)

	assert.NoError(t, err)
	assert.Equal(t, "bar", event.Kvs["foo"])
	assert.Equal(t, "myjob", event.Job)
	assert.Equal(t, "myevent", event.Event)
	assert.EqualValues(t, 3.14, event.Value)
}

func TestJsonWriterSinkEventComplete(t *testing.T) {
	var buf bytes.Buffer
	dec := json.NewDecoder(&buf)
	for kind, kindStr := range completionStatusToString {
		sink := JsonWriterSink{&buf}
		sink.EmitComplete("myjob", kind, 1204000, nil)

		event := &testJsonEvent{}
		err := dec.Decode(event)

		assert.NoError(t, err)

		assert.Equal(t, "myjob", event.Job)
		assert.Equal(t, kindStr, event.Status)
		assert.EqualValues(t, 1204000, event.Nanoseconds)
		buf.Reset()
	}
}

func BenchmarkJsonWriterSinkEmitBlankEvent(b *testing.B) {
	var buf bytes.Buffer
	sink := JsonWriterSink{&buf}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		sink.EmitEvent("myjob", "myevent", nil)
	}
	b.ReportAllocs()
}

func BenchmarkJsonWriterSinkEmitSmallEvent(b *testing.B) {
	var buf bytes.Buffer
	someKvs := map[string]string{"foo": "bar", "qux": "dog"}
	sink := JsonWriterSink{&buf}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		sink.EmitEvent("myjob", "myevent", someKvs)
	}
	b.ReportAllocs()
}
