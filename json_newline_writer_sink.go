package health

import "io"

type JsonNewlineWriterSink struct {
	jj *JsonWriterSink
}

func NewJsonNewlineWriterSink(w io.Writer) *JsonNewlineWriterSink {
	return &JsonNewlineWriterSink{&JsonWriterSink{w}}
}

func (j *JsonNewlineWriterSink) EmitEvent(job string, event string, kvs map[string]string) {
	j.jj.EmitEvent(job, event, kvs)
	j.jj.Write([]byte{'\n'})
}

func (j *JsonNewlineWriterSink) EmitEventErr(job string, event string, err error, kvs map[string]string) {
	j.jj.EmitEventErr(job, event, err, kvs)
	j.jj.Write([]byte{'\n'})
}

func (j *JsonNewlineWriterSink) EmitTiming(job string, event string, nanoseconds int64, kvs map[string]string) {
	j.jj.EmitTiming(job, event, nanoseconds, kvs)
	j.jj.Write([]byte{'\n'})
}

func (j *JsonNewlineWriterSink) EmitGauge(job string, event string, value float64, kvs map[string]string) {
	j.jj.EmitGauge(job, event, value, kvs)
	j.jj.Write([]byte{'\n'})
}

func (j *JsonNewlineWriterSink) EmitComplete(job string, status CompletionStatus, nanoseconds int64, kvs map[string]string) {
	j.jj.EmitComplete(job, status, nanoseconds, kvs)
	j.jj.Write([]byte{'\n'})
}
