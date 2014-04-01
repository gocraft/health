package health

import (
	"fmt"
	"io"
	"time"
)

type Kv map[string]interface{}

type Stream struct {
	Writer    io.Writer
	KeyValues map[string]string
}

type Job struct {
	Stream             *Stream
	JobName            string
	KeyValues          map[string]string
	NanosecondsAtStart int64
}

func NewStream(writer io.Writer) *Stream {
	return &Stream{
		Writer: writer,
	}
}

func (s *Stream) KeyValue(key string, value interface{}) *Stream {
	if s.KeyValues == nil {
		s.KeyValues = make(map[string]string)
	}

	s.KeyValues[key] = fmt.Sprint(value)

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

func (j *Job) KeyValue(key string, value interface{}) *Job {
	if j.KeyValues == nil {
		j.KeyValues = make(map[string]string)
	}

	j.KeyValues[key] = fmt.Sprint(value)

	return j
}

func (j *Job) Event(name string, kvs ...Kv) {

}

func (j *Job) Timing(name string, nanoseconds int64, kvs ...Kv) {
}

func (j *Job) Success() {

}

func (j *Job) UnhandledError() {

}

func (j *Job) ValidationError() {
	// TODO: unsure what arguments there are
}
