package health

import (
	"bytes"
	"fmt"
	"net"
	"time"
)

type StatsDSinkSantizationFunc func(string) string

// This sink emits to a StatsD deaemon by sending it a UDP packet.
type StatsDSink struct {
	SanitizationFunc StatsDSinkSantizationFunc

	conn net.Conn

	// Prefix is something like "metroid"
	// Events emitted to StatsD would be metroid.myevent.wat
	// Eg, don't include a trailing dot in the prefix.
	// It can be "", that's fine.
	prefix string
}

func NewStatsDSink(addr, prefix string) (Sink, error) {
	c, err := net.DialTimeout("udp", addr, 2*time.Second)
	if err != nil {
		return nil, err
	}

	sink := &StatsDSink{
		SanitizationFunc: sanitizeKey,
		conn:             c,
		prefix:           prefix,
	}

	return sink, nil
}

// If event is "my.event", and job is "cool.job"
// This will emit two events to statsd:
// "my.event"
// "cool.job.my.event"
// (this will apply prefix if it's set, so "prefix.my.event" and "prefix.cool.job.my.event")
func (s *StatsDSink) EmitEvent(job string, event string, kvs map[string]string) {
	key1, key2 := s.eventKeys(job, event, "")
	s.inc(key1)
	s.inc(key2)
}

// If event is "my.event", and job is "cool.job",
// This will emit two events: "my.event.error" and "cool.job.my.event.error" (prefix applied if present)
func (s *StatsDSink) EmitEventErr(job string, event string, inputErr error, kvs map[string]string) {
	key1, key2 := s.eventKeys(job, event, "error")
	s.inc(key1)
	s.inc(key2)
}

func (s *StatsDSink) EmitTiming(job string, event string, nanos int64, kvs map[string]string) {
	key1, key2 := s.eventKeys(job, event, "")
	s.measure(key1, nanos)
	s.measure(key2, nanos)
}

// if job is "my.job", this will emit "my.job.xyz" where xyz is "success", etc (see completionStatusToString)
func (s *StatsDSink) EmitComplete(job string, status CompletionStatus, nanos int64, kvs map[string]string) {
	var b bytes.Buffer

	if s.prefix != "" {
		b.WriteString(s.prefix)
		b.WriteRune('.')
	}
	b.WriteString(s.SanitizationFunc(job))
	b.WriteRune('.')
	b.WriteString(status.String())

	s.measure(b.String(), nanos)
}

func (s *StatsDSink) eventKeys(job, event, suffix string) (string, string) {
	var key1 bytes.Buffer // event
	var key2 bytes.Buffer // job.event

	if s.prefix != "" {
		key1.WriteString(s.prefix)
		key1.WriteRune('.')
		key2.WriteString(s.prefix)
		key2.WriteRune('.')
	}

	key1.WriteString(s.SanitizationFunc(event))
	key2.WriteString(s.SanitizationFunc(job))
	key2.WriteRune('.')
	key2.WriteString(s.SanitizationFunc(event))

	if suffix != "" {
		key1.WriteRune('.')
		key1.WriteString(suffix)
		key2.WriteRune('.')
		key2.WriteString(suffix)
	}

	return key1.String(), key2.String()
}

func (s *StatsDSink) inc(key string) {
	var msg bytes.Buffer
	msg.WriteString(key)
	msg.WriteString(":1|c\n")
	s.send(msg.Bytes())
}

func (s *StatsDSink) measure(key string, nanos int64) {
	var msg bytes.Buffer
	msg.WriteString(key)
	msg.WriteRune(':')
	msg.WriteString(fmt.Sprintf("%f", float64(nanos)/float64(time.Millisecond)))
	msg.WriteString("|ms\n")
	s.send(msg.Bytes())
}

func (s *StatsDSink) send(msg []byte) {
	s.conn.Write(msg)
}

func sanitizeKey(k string) string {
	var key bytes.Buffer
	for _, c := range k {
		if c == '|' || c == ':' {
			key.WriteRune('$')
		} else {
			key.WriteRune(c)
		}
	}
	return key.String()
}
