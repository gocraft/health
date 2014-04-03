package health

import (
	"io"
	"time"
)

// This is primarily used as syntactic sugar for libs outside this app for passing in maps easily.
// We don't rely on it internally b/c I don't want to tie interfaces to the 'health' package.
type Kvs map[string]string

type EventReceiver interface {
	Event(eventName string)
	EventKv(eventName string, kvs map[string]string)
	Timing(eventName string, nanoseconds int64)
	TimingKv(eventName string, nanoseconds int64, kvs map[string]string)
}

type Stream struct {
	Sinks     []Sink
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
	allKvs := j.mergedKeyValues(nil)
	for _, sink := range j.Stream.Sinks {
		sink.EmitEvent(j.JobName, eventName, allKvs)
	}
}

func (j *Job) EventKv(eventName string, kvs map[string]string) {
	allKvs := j.mergedKeyValues(kvs)
	for _, sink := range j.Stream.Sinks {
		sink.EmitEvent(j.JobName, eventName, allKvs)
	}
}

func (j *Job) Timing(eventName string, nanoseconds int64) {
	allKvs := j.mergedKeyValues(nil)
	for _, sink := range j.Stream.Sinks {
		sink.EmitTiming(j.JobName, eventName, nanoseconds, allKvs)
	}
}

func (j *Job) TimingKv(eventName string, nanoseconds int64, kvs map[string]string) {
	allKvs := j.mergedKeyValues(kvs)
	for _, sink := range j.Stream.Sinks {
		sink.EmitTiming(j.JobName, eventName, nanoseconds, allKvs)
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

func (j *Job) mergedKeyValues(instanceKvs map[string]string) map[string]string {
	var allKvs map[string]string

	// Count how many maps actually have contents in them. If it's 0 or 1, we won't allocate a new map.
	// Also, optimistically set allKvs. We might use it or we might overwrite the value with a newly made map.
	var kvCount = 0
	if len(j.KeyValues) > 0 {
		kvCount += 1
		allKvs = j.KeyValues
	}
	if len(j.Stream.KeyValues) > 0 {
		kvCount += 1
		allKvs = j.Stream.KeyValues
	}
	if len(instanceKvs) > 0 {
		kvCount += 1
		allKvs = instanceKvs
	}

	if kvCount > 1 {
		allKvs = make(map[string]string)
		for k, v := range j.Stream.KeyValues {
			allKvs[k] = v
		}
		for k, v := range j.KeyValues {
			allKvs[k] = v
		}
		for k, v := range instanceKvs {
			allKvs[k] = v
		}
	}

	return allKvs
}
