package health

import (
	"encoding/json"
	"io"
)

type JsonSink struct {
	io.Writer
}

func (j *JsonSink) EmitEvent(job string, event string, kvs map[string]string) {

	b, _ := json.Marshal(struct {
		Job       string            `json:"event"`
		Event     string            `json:"job"`
		Timestamp string            `json:"timestamp"`
		Kvs       map[string]string `json:"kvs"`
	}{job, event, timestamp(), kvs})
	j.Write(b)
}

func (j *JsonSink) EmitEventErr(job string, event string, err error, kvs map[string]string) {

	b, _ := json.Marshal(struct {
		Job       string            `json:"event"`
		Event     string            `json:"job"`
		Timestamp string            `json:"timestamp"`
		Err       error             `json:"err"`
		Kvs       map[string]string `json:"kvs"`
	}{job, event, timestamp(), err, kvs})
	j.Write(b)
}

func (j *JsonSink) EmitTiming(job string, event string, nanoseconds int64, kvs map[string]string) {

	b, _ := json.Marshal(struct {
		Job         string            `json:"event"`
		Event       string            `json:"job"`
		Timestmap   string            `json:"timestamp"`
		Nanoseconds int64             `json:"nanoseconds"`
		Kvs         map[string]string `json:"kvs"`
	}{job, event, timestamp(), nanoseconds, kvs})
	j.Write(b)
}

func (j *JsonSink) EmitComplete(job string, status CompletionStatus, nanoseconds int64, kvs map[string]string) {

	b, _ := json.Marshal(struct {
		Job         string            `json:"event"`
		Status      string            `json:"status"`
		Timestmap   string            `json:"timestamp"`
		Nanoseconds int64             `json:"nanoseconds"`
		Kvs         map[string]string `json:"kvs"`
	}{job, status.String(), timestamp(), nanoseconds, kvs})
	j.Write(b)
}
