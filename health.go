package health

import (
	"io"
	"time"
)


type Stream struct {
	Sinks []Sink
	KeyValues map[string]string
}

type Job struct {
	Stream             *Stream
	JobName            string
	KeyValues          map[string]string
	NanosecondsAtStart int64
	// Thought: auto generate a job-id
}

func NewStream() *Stream {
	return &Stream{}
}

func (s *Stream) AddLogfileWriterSink(writer io.Writer) *Stream {
	s.Sinks = append(s.Sinks, &LogfileWriterSink{Writer: writer})
	return s
}

func (s *Stream) KeyValue(key string, value string) *Stream {
	if s.KeyValues == nil {
		s.KeyValues = make(map[string]string)
	}
	s.KeyValues[key] = value
	return s
}

// Returns a NEW Stream. NOTE: the job name will completely overwrite the
func (s *Stream) Job(name string) *Job {
	return &Job{
		Stream:             s,
		JobName:            name,
		NanosecondsAtStart: time.Now().UnixNano(),
	}
}

func (j *Job) KeyValue(key string, value string) *Job {
	if j.KeyValues == nil {
		j.KeyValues = make(map[string]string)
	}
	j.KeyValues[key] = value
	return j
}

func (j *Job) Event(eventName string) {
	for _, sink := range j.Stream.Sinks {
		sink.EmitEvent(j.JobName, eventName, nil)
	}
}

func (j *Job) EventKv(eventName string, kvs map[string]string) {
	for _, sink := range j.Stream.Sinks {
		sink.EmitEvent(j.JobName, eventName, kvs)
	}
}

func (j *Job) Timing(eventName string, nanoseconds int64) {
	for _, sink := range j.Stream.Sinks {
		sink.EmitTiming(j.JobName, eventName, nanoseconds, nil)
	}
}

func (j *Job) TimingKv(eventName string, nanoseconds int64, kvs map[string]string) {
	for _, sink := range j.Stream.Sinks {
		sink.EmitTiming(j.JobName, eventName, nanoseconds, kvs)
	}
}

func (j *Job) Success() {
	j.Event("success")
}

func (j *Job) UnhandledError() {
	j.Event("error")
}

func (j *Job) ValidationError() {
	j.Event("validation")
}
