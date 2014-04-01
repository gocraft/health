package health

import (
	"io"
	"fmt"
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
	
	var kvString string
	if kvs != nil {} // we really want to print this in alphabetical order to keep entries consistent.
	
	fmt.Fprintf(s.Writer, "[%s]: %s - %s ", dateString, job, event, kvString)
	
	return nil
}