package health

import (
	"testing"
	"bytes"
	"github.com/stretchr/testify/assert"
	"regexp"
	"errors"
)

var basicEventRegexp = regexp.MustCompile("\\[[^\\]]+\\]: job:(.+) event:(.+)")
var kvsEventRegexp = regexp.MustCompile("\\[[^\\]]+\\]: job:(.+) event:(.+) kvs:\\[(.+)\\]")
var basicEventErrRegexp = regexp.MustCompile("\\[[^\\]]+\\]: job:(.+) event:(.+) err:(.+)")
var kvsEventErrRegexp = regexp.MustCompile("\\[[^\\]]+\\]: job:(.+) event:(.+) err:(.+) kvs:\\[(.+)\\]")
var basicTimingRegexp = regexp.MustCompile("\\[[^\\]]+\\]: job:(.+) event:(.+) time:(.+)")
var kvsTimingRegexp = regexp.MustCompile("\\[[^\\]]+\\]: job:(.+) event:(.+) time:(.+) kvs:\\[(.+)\\]")

var testErr = errors.New("my test error")

func TestLogfileSinkEmitEventBasic(t *testing.T) {
	var b bytes.Buffer
	sink := LogfileWriterSink{Writer: &b}
	err := sink.EmitEvent("myjob", "myevent", nil)
	assert.NoError(t, err)
	
	str := b.String()
	
	result := basicEventRegexp.FindStringSubmatch(str)
	assert.Equal(t, 3, len(result))
	assert.Equal(t, "myjob", result[1])
	assert.Equal(t, "myevent", result[2])
}

func TestLogfileSinkEmitEventKvs(t *testing.T) {
	var b bytes.Buffer
	sink := LogfileWriterSink{Writer: &b}
	err := sink.EmitEvent("myjob", "myevent", map[string]string{"wat": "ok", "another": "thing"})
	assert.NoError(t, err)
	
	str := b.String()
	
	result := kvsEventRegexp.FindStringSubmatch(str)
	assert.Equal(t, 4, len(result))
	assert.Equal(t, "myjob", result[1])
	assert.Equal(t, "myevent", result[2])
	assert.Equal(t, "another:thing wat:ok", result[3])
}

func TestLogfileSinkEmitEventErrBasic(t *testing.T) {
	var b bytes.Buffer
	sink := LogfileWriterSink{Writer: &b}
	err := sink.EmitEventErr("myjob", "myevent", testErr, nil)
	assert.NoError(t, err)
	
	str := b.String()
	
	result := basicEventErrRegexp.FindStringSubmatch(str)
	assert.Equal(t, 4, len(result))
	assert.Equal(t, "myjob", result[1])
	assert.Equal(t, "myevent", result[2])
	assert.Equal(t, testErr.Error(), result[3])
}

func TestLogfileSinkEmitEventErrKvs(t *testing.T) {
	var b bytes.Buffer
	sink := LogfileWriterSink{Writer: &b}
	err := sink.EmitEventErr("myjob", "myevent", testErr, map[string]string{"wat": "ok", "another": "thing"})
	assert.NoError(t, err)
	
	str := b.String()
	
	result := kvsEventErrRegexp.FindStringSubmatch(str)
	assert.Equal(t, 5, len(result))
	assert.Equal(t, "myjob", result[1])
	assert.Equal(t, "myevent", result[2])
	assert.Equal(t, testErr.Error(), result[3])
	assert.Equal(t, "another:thing wat:ok", result[4])
}


func TestLogfileSinkEmitTimingBasic(t *testing.T) {
	var b bytes.Buffer
	sink := LogfileWriterSink{Writer: &b}
	err := sink.EmitTiming("myjob", "myevent", 1204000, nil)
	assert.NoError(t, err)
	
	str := b.String()
	
	result := basicTimingRegexp.FindStringSubmatch(str)
	assert.Equal(t, 4, len(result))
	assert.Equal(t, "myjob", result[1])
	assert.Equal(t, "myevent", result[2])
	assert.Equal(t, "1204 μs", result[3])
}

func TestLogfileSinkEmitTimingKvs(t *testing.T) {
	var b bytes.Buffer
	sink := LogfileWriterSink{Writer: &b}
	err := sink.EmitTiming("myjob", "myevent", 34567890, map[string]string{"wat": "ok", "another": "thing"})
	assert.NoError(t, err)
	
	str := b.String()
	
	result := kvsTimingRegexp.FindStringSubmatch(str)
	assert.Equal(t, 5, len(result))
	assert.Equal(t, "myjob", result[1])
	assert.Equal(t, "myevent", result[2])
	assert.Equal(t, "34 ms", result[3])
	assert.Equal(t, "another:thing wat:ok", result[4])
}

