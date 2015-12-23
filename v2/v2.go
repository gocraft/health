package v2

import (
	"bytes"
	"github.com/gocraft/health"
	"net"
	"strconv"
	"time"
)

// TODO:
// - better sanitization func
// - tests
//   - periodic purge
//   - 1440 limit
//   -
//   -

type SanitizationFunc func(string) string

type eventKey struct {
	job    string
	event  string
	suffix string
}

type prefixBuffer struct {
	*bytes.Buffer
	prefixLen int
}

type StatsDSink struct {
	SanitizationFunc SanitizationFunc

	cmdChan       chan emitCmd
	drainDoneChan chan int
	stopDoneChan  chan int

	// Prefix is something like "metroid"
	// Events emitted to StatsD would be metroid.myevent.wat
	// Eg, don't include a trailing dot in the prefix.
	// It can be "", that's fine.
	prefix      string
	flushPeriod time.Duration

	udpBuf    bytes.Buffer
	timingBuf []byte

	udpConn *net.UDPConn
	udpAddr *net.UDPAddr

	// map of {job,event,suffix} -> entire byte slice to send to statsd
	// (suffix is "error" for EventErr)
	events map[eventKey][]byte

	// map of {job,event,suffix} to a re-usable buffer prefixed with the key.
	// Since each timing has a unique component (the time), we'll truncate to the prefix, write the timing,
	// and write the statsD suffix (eg, "|ms\n"). Then copy that to the UDP buffer.
	timingsAndGauges map[eventKey]prefixBuffer
}

type cmdKind int

const (
	cmdKindEvent cmdKind = iota
	cmdKindEventErr
	cmdKindTiming
	cmdKindGauge
	cmdKindComplete
	cmdKindFlush
	cmdKindDrain
	cmdKindStop
)

type emitCmd struct {
	Kind   cmdKind
	Job    string
	Event  string
	Nanos  int64
	Value  float64
	Status health.CompletionStatus
}

const cmdChanBuffSize = 8192 // random-ass-guess
const maxUdpBytes = 1440     // 1500(Ethernet MTU) - 60(Max UDP header size

func NewStatsDSink(addr, prefix string) (*StatsDSink, error) {
	c, err := net.ListenPacket("udp", ":0")
	if err != nil {
		return nil, err
	}

	ra, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}

	s := &StatsDSink{
		SanitizationFunc: sanitizeKey,
		udpConn:          c.(*net.UDPConn),
		udpAddr:          ra,
		cmdChan:          make(chan emitCmd, cmdChanBuffSize),
		drainDoneChan:    make(chan int),
		stopDoneChan:     make(chan int),
		prefix:           prefix,
		flushPeriod:      100 * time.Millisecond,
		events:           make(map[eventKey][]byte),
		timingsAndGauges: make(map[eventKey]prefixBuffer),
	}

	go s.loop()

	return s, nil
}

func (s *StatsDSink) Stop() {
	s.cmdChan <- emitCmd{Kind: cmdKindStop}
	<-s.stopDoneChan
}

func (s *StatsDSink) Drain() {
	s.cmdChan <- emitCmd{Kind: cmdKindDrain}
	<-s.drainDoneChan
}

func (s *StatsDSink) EmitEvent(job string, event string, kvs map[string]string) {
	s.cmdChan <- emitCmd{Kind: cmdKindEvent, Job: job, Event: event}
}

func (s *StatsDSink) EmitEventErr(job string, event string, inputErr error, kvs map[string]string) {
	s.cmdChan <- emitCmd{Kind: cmdKindEventErr, Job: job, Event: event}
}

func (s *StatsDSink) EmitTiming(job string, event string, nanos int64, kvs map[string]string) {
	s.cmdChan <- emitCmd{Kind: cmdKindTiming, Job: job, Event: event, Nanos: nanos}
}

func (s *StatsDSink) EmitGauge(job string, event string, value float64, kvs map[string]string) {
	s.cmdChan <- emitCmd{Kind: cmdKindGauge, Job: job, Event: event, Value: value}
}

func (s *StatsDSink) EmitComplete(job string, status health.CompletionStatus, nanos int64, kvs map[string]string) {
	s.cmdChan <- emitCmd{Kind: cmdKindComplete, Job: job, Status: status, Nanos: nanos}
}

func (s *StatsDSink) loop() {
	cmdChan := s.cmdChan

	ticker := time.NewTicker(s.flushPeriod)
	go func() {
		for _ = range ticker.C {
			cmdChan <- emitCmd{Kind: cmdKindFlush}
		}
	}()

LOOP:
	for cmd := range cmdChan {
		switch cmd.Kind {
		case cmdKindDrain:
		DRAIN_LOOP:
			for {
				select {
				case cmd := <-cmdChan:
					s.processCmd(&cmd)
				default:
					s.flush()
					s.drainDoneChan <- 1
					break DRAIN_LOOP
				}
			}
		case cmdKindStop:
			s.stopDoneChan <- 1
			break LOOP
		case cmdKindFlush:
			s.flush()
		default:
			s.processCmd(&cmd)
		}
	}

	ticker.Stop()
}

func (s *StatsDSink) processCmd(cmd *emitCmd) {
	switch cmd.Kind {
	case cmdKindEvent:
		s.processEvent(cmd.Job, cmd.Event)
	case cmdKindEventErr:
		s.processEventErr(cmd.Job, cmd.Event)
	case cmdKindTiming:
		s.processTiming(cmd.Job, cmd.Event, cmd.Nanos)
	case cmdKindGauge:
		s.processGauge(cmd.Job, cmd.Event, cmd.Value)
	case cmdKindComplete:
		s.processComplete(cmd.Job, cmd.Status, cmd.Nanos)
	}
}

func (s *StatsDSink) processEvent(job string, event string) {
	b1, b2 := s.eventBytes(job, event, "")
	s.writeStatsDMetric(b1)
	s.writeStatsDMetric(b2)
}

func (s *StatsDSink) processEventErr(job string, event string) {
	b1, b2 := s.eventBytes(job, event, "error")
	s.writeStatsDMetric(b1)
	s.writeStatsDMetric(b2)
}

func (s *StatsDSink) processTiming(job string, event string, nanos int64) {
	b1, b2 := s.timingBytes(job, event, nanos)
	s.writeStatsDMetric(b1)
	s.writeStatsDMetric(b2)
}

func (s *StatsDSink) processGauge(job string, event string, value float64) {
	b1, b2 := s.gaugeBytes(job, event, value)
	s.writeStatsDMetric(b1)
	s.writeStatsDMetric(b2)
}

func (s *StatsDSink) processComplete(job string, status health.CompletionStatus, nanos int64) {
	s.writeNanosToTimingBuf(nanos)

	statusString := status.String()
	key := eventKey{job, "", statusString}
	b, ok := s.timingsAndGauges[key]
	if !ok {
		b.Buffer = &bytes.Buffer{}
		s.writeSanitizedKeys(b.Buffer, s.prefix, job, statusString)
		b.WriteByte(':')
		b.prefixLen = b.Len()

		// 123456789.99|ms\n 16 bytes. timing value represents 11 days max
		b.Grow(16)
		s.timingsAndGauges[key] = b
	}

	b.Truncate(b.prefixLen)
	b.Write(s.timingBuf)
	b.WriteString("|ms\n")

	s.writeStatsDMetric(b.Bytes())
}

func (s *StatsDSink) flush() {
	if s.udpBuf.Len() > 0 {
		s.udpConn.WriteToUDP(s.udpBuf.Bytes(), s.udpAddr)
		s.udpBuf.Truncate(0)
	}
}

// assumes b is a well-formed statsd metric like "job.event:1|c\n" (including newline)
func (s *StatsDSink) writeStatsDMetric(b []byte) {
	lenb := len(b)
	lenUdpBuf := s.udpBuf.Len()

	// single metric exceeds limit. sad day.
	if lenb > maxUdpBytes {
		return
	}

	if (lenb + lenUdpBuf) > maxUdpBytes {
		s.udpConn.WriteToUDP(s.udpBuf.Bytes(), s.udpAddr)
		s.udpBuf.Truncate(0)
	}

	s.udpBuf.Write(b)
}

// returns {bytes for event, bytes for job+event}
func (s *StatsDSink) eventBytes(job, event, suffix string) ([]byte, []byte) {
	key := eventKey{job, event, suffix}

	jobEventBytes, ok := s.events[key]
	if !ok {
		var b bytes.Buffer
		s.writeSanitizedKeys(&b, s.prefix, job, event, suffix)
		b.WriteString(":1|c\n")

		jobEventBytes = b.Bytes()
		s.events[key] = jobEventBytes
	}

	key.job = ""
	eventBytes, ok := s.events[key]
	if !ok {
		var b bytes.Buffer
		s.writeSanitizedKeys(&b, s.prefix, event, suffix)
		b.WriteString(":1|c\n")

		eventBytes = b.Bytes()
		s.events[key] = eventBytes
	}

	return eventBytes, jobEventBytes
}

func (s *StatsDSink) timingBytes(job, event string, nanos int64) ([]byte, []byte) {
	s.writeNanosToTimingBuf(nanos)

	key := eventKey{job, event, ""}
	b, ok := s.timingsAndGauges[key]
	if !ok {
		b.Buffer = &bytes.Buffer{}
		s.writeSanitizedKeys(b.Buffer, s.prefix, job, event)
		b.WriteByte(':')
		b.prefixLen = b.Len()

		// 123456789.99|ms\n 16 bytes. timing value represents 11 days max
		b.Grow(16)
		s.timingsAndGauges[key] = b
	}

	b.Truncate(b.prefixLen)
	b.Write(s.timingBuf)
	b.WriteString("|ms\n")
	jobEventBytes := b.Bytes()

	key.job = ""
	b, ok = s.timingsAndGauges[key]
	if !ok {
		b.Buffer = &bytes.Buffer{}
		s.writeSanitizedKeys(b.Buffer, s.prefix, event)
		b.WriteByte(':')
		b.prefixLen = b.Len()

		// 123456789.99|ms\n 16 bytes. timing value represents 11 days max
		b.Grow(16)
		s.timingsAndGauges[key] = b
	}

	b.Truncate(b.prefixLen)
	b.Write(s.timingBuf)
	b.WriteString("|ms\n")
	eventBytes := b.Bytes()

	return eventBytes, jobEventBytes
}

func (s *StatsDSink) gaugeBytes(job, event string, value float64) ([]byte, []byte) {
	s.timingBuf = s.timingBuf[0:0]
	s.timingBuf = strconv.AppendFloat(s.timingBuf, value, 'f', 2, 64)

	key := eventKey{job, event, ""}
	b, ok := s.timingsAndGauges[key]
	if !ok {
		b.Buffer = &bytes.Buffer{}
		s.writeSanitizedKeys(b.Buffer, s.prefix, job, event)
		b.WriteByte(':')
		b.prefixLen = b.Len()

		// 123456789.99|ms\n 16 bytes. timing value represents 11 days max
		b.Grow(16)
		s.timingsAndGauges[key] = b
	}

	b.Truncate(b.prefixLen)
	b.Write(s.timingBuf)
	b.WriteString("|g\n")
	jobEventBytes := b.Bytes()

	key.job = ""
	b, ok = s.timingsAndGauges[key]
	if !ok {
		b.Buffer = &bytes.Buffer{}
		s.writeSanitizedKeys(b.Buffer, s.prefix, event)
		b.WriteByte(':')
		b.prefixLen = b.Len()

		// 123456789.99|ms\n 16 bytes. timing value represents 11 days max
		b.Grow(16)
		s.timingsAndGauges[key] = b
	}

	b.Truncate(b.prefixLen)
	b.Write(s.timingBuf)
	b.WriteString("|g\n")
	eventBytes := b.Bytes()

	return eventBytes, jobEventBytes
}

func (s *StatsDSink) writeSanitizedKeys(b *bytes.Buffer, keys ...string) {
	needDot := false
	for _, k := range keys {
		if k != "" {
			if needDot {
				b.WriteByte('.')
			}
			b.WriteString(s.SanitizationFunc(k))
			needDot = true
		}
	}
}

func (s *StatsDSink) writeNanosToTimingBuf(nanos int64) {
	s.timingBuf = s.timingBuf[0:0]
	if nanos >= 10e6 {
		// More than 10 milliseconds. We'll just print as an integer
		s.timingBuf = strconv.AppendInt(s.timingBuf, nanos/1e6, 10)
	} else {
		s.timingBuf = strconv.AppendFloat(s.timingBuf, float64(nanos)/float64(time.Millisecond), 'f', 2, 64)
	}
}

func sanitizeKey(k string) string {
	var key bytes.Buffer
	for _, c := range k {
		if c == '|' || c == ':' {
			key.WriteByte('$')
		} else {
			key.WriteRune(c)
		}
	}
	return key.String()
}
