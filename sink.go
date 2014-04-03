package health

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"time"
)

type Sink interface {
	EmitEvent(job string, event string, kvs map[string]string) error
	EmitTiming(job string, event string, nanoseconds int64, kvs map[string]string) error
}

// This sink writes bytes in a format that a human might like to read in a logfile
type LogfileWriterSink struct {
	Writer io.Writer
}

// Thought: JSONWriterSink, SyslogSink, etc
// Thought: LibratoSink
// Thought: InternalSink

func (s *LogfileWriterSink) EmitEvent(job string, event string, kvs map[string]string) error {
	datetime := time.Now().UTC().Format(time.RFC3339Nano)
	kvString := consistentSerializeMap(kvs)
	_, err := fmt.Fprintf(s.Writer, "[%s]: %s - %s [%s]\n", datetime, job, event, kvString)
	return err
}

func (s *LogfileWriterSink) EmitTiming(job string, event string, nanos int64, kvs map[string]string) error {
	datetime := time.Now().UTC().Format(time.RFC3339Nano)
	kvString := consistentSerializeMap(kvs)
	duration := formatNanoseconds(nanos)
	_, err := fmt.Fprintf(s.Writer, "[%s]: %s - %s: %s [%s]\n", datetime, job, event, duration, kvString)
	return err
}

// {} -> ""
// {"abc": "def", "foo": "bar"} -> "abc:def foo:bar"
// NOTE: map keys are outputted in sorted order
func consistentSerializeMap(kvs map[string]string) string {
	if len(kvs) == 0 {
		return ""
	}

	var keys []string
	for k := range kvs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	keysLenMinusOne := len(keys) - 1

	var b bytes.Buffer

	for i, k := range keys {
		b.WriteString(k)
		b.WriteRune(':')
		b.WriteString(kvs[k])

		if i != keysLenMinusOne {
			b.WriteRune(' ')
		}
	}

	return b.String()
}

func formatNanoseconds(duration int64) string {
	var durationUnits string
	switch {
	case duration > 2000000:
		durationUnits = "ms"
		duration /= 1000000
	case duration > 2000:
		durationUnits = "μs"
		duration /= 1000
	default:
		durationUnits = "ns"
	}

	return fmt.Sprintf("%d %s", duration, durationUnits)
}
