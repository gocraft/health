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
	EventErr(eventName string, err error) error
	EventErrKv(eventName string, err error, kvs map[string]string) error
	Timing(eventName string, nanoseconds int64)
	TimingKv(eventName string, nanoseconds int64, kvs map[string]string)
}

// Thought: add
// ErrorEvent(eventName string) error
// and ErrorEventKv, (not sure about Timing as well)
// So if a function has an error and wants to log it, they can do this:
// if err != nil {
// 	   return Stram.ErrorEventKv("bad_data", Kvs{"err": err})
// }
// Or, one could name it ErrorEvent(eventName string, err error) error, which would automatically add err as a kv thing and then return the error

type Stream struct {
	Sinks     []Sink
	KeyValues map[string]string
	*Job
}

type Job struct {
	Stream    *Stream
	JobName   string
	KeyValues map[string]string
	Start     time.Time
}

func NewStream() *Stream {
	s := &Stream{}
	s.Job = s.NewJob("general")
	return s
}

func (s *Stream) AddWriterSink(writer io.Writer) *Stream {
	s.Sinks = append(s.Sinks, &WriterSink{writer})
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
func (s *Stream) NewJob(name string) *Job {
	return &Job{
		Stream:  s,
		JobName: name,
		Start:   time.Now(),
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

func (j *Job) EventErr(eventName string, err error) error {
	var kvs Kvs
	if err != nil {
		kvs = Kvs{"err": err.Error()}
	}
	allKvs := j.mergedKeyValues(kvs)
	for _, sink := range j.Stream.Sinks {
		sink.EmitEvent(j.JobName, eventName, allKvs)
	}
	return err
}

func (j *Job) EventErrKv(eventName string, err error, kvs map[string]string) error {
	if kvs == nil {
		kvs = make(Kvs)
	}
	if err != nil {
		kvs["err"] = err.Error()
	}
	allKvs := j.mergedKeyValues(kvs)
	for _, sink := range j.Stream.Sinks {
		sink.EmitEvent(j.JobName, eventName, allKvs)
	}
	return err
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
	j.Timing("success", time.Since(j.Start).Nanoseconds())
}

func (j *Job) SuccessKv(kvs map[string]string) {
	j.TimingKv("success", time.Since(j.Start).Nanoseconds(), kvs)
}

func (j *Job) UnhandledError() {
	j.Timing("error", time.Since(j.Start).Nanoseconds())
}

func (j *Job) UnhandledErrorKv(kvs map[string]string) {
	j.TimingKv("error", time.Since(j.Start).Nanoseconds(), kvs)
}

func (j *Job) ValidationError() {
	j.Timing("validation", time.Since(j.Start).Nanoseconds())
}

func (j *Job) ValidationErrorKv(kvs map[string]string) {
	j.TimingKv("validation", time.Since(j.Start).Nanoseconds(), kvs)
}

func (j *Job) JunkError() {
	j.Timing("junk", time.Since(j.Start).Nanoseconds())
}

func (j *Job) JunkErrorKv(kvs map[string]string) {
	j.TimingKv("junk", time.Since(j.Start).Nanoseconds(), kvs)
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
