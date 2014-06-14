package health

import (
	"bytes"
	"fmt"
	"sort"
)

type CompletionType int

const (
	Success CompletionType = iota
	ValidationError
	Panic
	Error
	Junk
)

var completionTypeToString = map[CompletionType]string{
	Success:         "success",
	ValidationError: "validation_error",
	Panic:           "panic",
	Error:           "error",
	Junk:            "junk",
}

type Sink interface {
	EmitEvent(job string, event string, kvs map[string]string) error
	EmitEventErr(job string, event string, err error, kvs map[string]string) error
	EmitTiming(job string, event string, nanoseconds int64, kvs map[string]string) error
	EmitJobCompletion(job string, kind CompletionType, nanoseconds int64, kvs map[string]string) error
}

// {} -> ""
// {"abc": "def", "foo": "bar"} -> " [abc:def foo:bar]"
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
	b.WriteString(" kvs:[")
	for i, k := range keys {
		b.WriteString(k)
		b.WriteRune(':')
		b.WriteString(kvs[k])

		if i != keysLenMinusOne {
			b.WriteRune(' ')
		}
	}
	b.WriteRune(']')
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
