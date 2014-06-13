package health

import (
	"fmt"
	"io"
	"time"
)

// This sink writes bytes in a format that a human might like to read in a logfile
type LogfileWriterSink struct {
	Writer io.Writer
}

func (s *LogfileWriterSink) EmitEvent(job string, event string, kvs map[string]string) error {
	datetime := timestamp()
	kvString := consistentSerializeMap(kvs)
	_, err := fmt.Fprintf(s.Writer, "[%s]: job:%s event:%s%s\n", datetime, job, event, kvString)
	return err
}

func (s *LogfileWriterSink) EmitEventErr(job string, event string, inputErr error, kvs map[string]string) error {
	datetime := timestamp()
	kvString := consistentSerializeMap(kvs)
	_, err := fmt.Fprintf(s.Writer, "[%s]: job:%s event:%s err:%s%s\n", datetime, job, event, inputErr.Error(), kvString)
	return err
}

func (s *LogfileWriterSink) EmitTiming(job string, event string, nanos int64, kvs map[string]string) error {
	datetime := timestamp()
	kvString := consistentSerializeMap(kvs)
	duration := formatNanoseconds(nanos)
	_, err := fmt.Fprintf(s.Writer, "[%s]: job:%s event:%s time:%s%s\n", datetime, job, event, duration, kvString)
	return err
}

func (s *LogfileWriterSink) EmitJobCompletion(job string, kind CompletionType, nanoseconds int64, kvs map[string]string) error {
	return nil
}

func timestamp() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}