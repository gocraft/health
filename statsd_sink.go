package health

import (
	"bytes"
	"net"
	"strconv"
	"time"
)

// TODO:
// - tests
//   - periodic purge
//   - 1440 limit
// - benchmark for event
// - option for single emit

type StatsDSinkSanitizationFunc func(*bytes.Buffer, string)

type eventKey struct {
	job    string
	event  string
	suffix string
}

type prefixBuffer struct {
	*bytes.Buffer
	prefixLen int
}

type StatsDSinkOptions struct {
	// Prefix is something like "metroid"
	// Events emitted to StatsD would be metroid.myevent.wat
	// Eg, don't include a trailing dot in the prefix.
	// It can be "", that's fine.
	Prefix string

	// SanitizationFunc sanitizes jobs and events before sending them to statsd
	SanitizationFunc StatsDSinkSanitizationFunc

	// SkipNestedEvents will skip {events,timers,gauges} from sending the job.event version
	// and will only send the event version.
	SkipNestedEvents bool

	// SkipTopLevelEvents will skip {events,timers,gauges} from sending the event version
	// and will only send the job.event version.
	SkipTopLevelEvents bool
}

var defaultStatsDOptions = StatsDSinkOptions{SanitizationFunc: sanitizeKey}

type StatsDSink struct {
	options StatsDSinkOptions

	cmdChan       chan statsdEmitCmd
	drainDoneChan chan struct{}
	stopDoneChan  chan struct{}

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

type statsdCmdKind int

const (
	statsdCmdKindEvent statsdCmdKind = iota
	statsdCmdKindEventErr
	statsdCmdKindTiming
	statsdCmdKindGauge
	statsdCmdKindComplete
	statsdCmdKindFlush
	statsdCmdKindDrain
	statsdCmdKindStop
)

type statsdEmitCmd struct {
	Kind   statsdCmdKind
	Job    string
	Event  string
	Nanos  int64
	Value  float64
	Status CompletionStatus
}

const cmdChanBuffSize = 8192 // random-ass-guess
const maxUdpBytes = 1440     // 1500(Ethernet MTU) - 60(Max UDP header size

func NewStatsDSink(addr string, options *StatsDSinkOptions) (*StatsDSink, error) {
	c, err := net.ListenPacket("udp", ":0")
	if err != nil {
		return nil, err
	}

	ra, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}

	s := &StatsDSink{
		udpConn:          c.(*net.UDPConn),
		udpAddr:          ra,
		cmdChan:          make(chan statsdEmitCmd, cmdChanBuffSize),
		drainDoneChan:    make(chan struct{}),
		stopDoneChan:     make(chan struct{}),
		flushPeriod:      100 * time.Millisecond,
		events:           make(map[eventKey][]byte),
		timingsAndGauges: make(map[eventKey]prefixBuffer),
	}

	if options != nil {
		s.options = *options
		if s.options.SanitizationFunc == nil {
			s.options.SanitizationFunc = sanitizeKey
		}
	} else {
		s.options = defaultStatsDOptions
	}

	go s.loop()

	return s, nil
}

func (s *StatsDSink) Stop() {
	s.cmdChan <- statsdEmitCmd{Kind: statsdCmdKindStop}
	<-s.stopDoneChan
}

func (s *StatsDSink) Drain() {
	s.cmdChan <- statsdEmitCmd{Kind: statsdCmdKindDrain}
	<-s.drainDoneChan
}

func (s *StatsDSink) EmitEvent(job string, event string, kvs map[string]string) {
	s.cmdChan <- statsdEmitCmd{Kind: statsdCmdKindEvent, Job: job, Event: event}
}

func (s *StatsDSink) EmitEventErr(job string, event string, inputErr error, kvs map[string]string) {
	s.cmdChan <- statsdEmitCmd{Kind: statsdCmdKindEventErr, Job: job, Event: event}
}

func (s *StatsDSink) EmitTiming(job string, event string, nanos int64, kvs map[string]string) {
	s.cmdChan <- statsdEmitCmd{Kind: statsdCmdKindTiming, Job: job, Event: event, Nanos: nanos}
}

func (s *StatsDSink) EmitGauge(job string, event string, value float64, kvs map[string]string) {
	s.cmdChan <- statsdEmitCmd{Kind: statsdCmdKindGauge, Job: job, Event: event, Value: value}
}

func (s *StatsDSink) EmitComplete(job string, status CompletionStatus, nanos int64, kvs map[string]string) {
	s.cmdChan <- statsdEmitCmd{Kind: statsdCmdKindComplete, Job: job, Status: status, Nanos: nanos}
}

func (s *StatsDSink) loop() {
	cmdChan := s.cmdChan

	ticker := time.NewTicker(s.flushPeriod)
	go func() {
		for _ = range ticker.C {
			cmdChan <- statsdEmitCmd{Kind: statsdCmdKindFlush}
		}
	}()

LOOP:
	for cmd := range cmdChan {
		switch cmd.Kind {
		case statsdCmdKindDrain:
		DRAIN_LOOP:
			for {
				select {
				case cmd := <-cmdChan:
					s.processCmd(&cmd)
				default:
					s.flush()
					s.drainDoneChan <- struct{}{}
					break DRAIN_LOOP
				}
			}
		case statsdCmdKindStop:
			s.stopDoneChan <- struct{}{}
			break LOOP
		case statsdCmdKindFlush:
			s.flush()
		default:
			s.processCmd(&cmd)
		}
	}

	ticker.Stop()
}

func (s *StatsDSink) processCmd(cmd *statsdEmitCmd) {
	switch cmd.Kind {
	case statsdCmdKindEvent:
		s.processEvent(cmd.Job, cmd.Event)
	case statsdCmdKindEventErr:
		s.processEventErr(cmd.Job, cmd.Event)
	case statsdCmdKindTiming:
		s.processTiming(cmd.Job, cmd.Event, cmd.Nanos)
	case statsdCmdKindGauge:
		s.processGauge(cmd.Job, cmd.Event, cmd.Value)
	case statsdCmdKindComplete:
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

func (s *StatsDSink) processComplete(job string, status CompletionStatus, nanos int64) {
	s.writeNanosToTimingBuf(nanos)

	statusString := status.String()
	key := eventKey{job, "", statusString}
	b, ok := s.timingsAndGauges[key]
	if !ok {
		b.Buffer = &bytes.Buffer{}
		s.writeSanitizedKeys(b.Buffer, s.options.Prefix, job, statusString)
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

	// This can happen based on the design of the system -- if we choose not to send a metric, we'll use nil.
	if lenb == 0 {
		return
	}

	// single metric exceeds limit. sad day.
	if lenb > maxUdpBytes {
		return
	}

	lenUdpBuf := s.udpBuf.Len()

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
		s.writeSanitizedKeys(&b, s.options.Prefix, job, event, suffix)
		b.WriteString(":1|c\n")

		jobEventBytes = b.Bytes()
		s.events[key] = jobEventBytes
	}

	key.job = ""
	eventBytes, ok := s.events[key]
	if !ok {
		var b bytes.Buffer
		s.writeSanitizedKeys(&b, s.options.Prefix, event, suffix)
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
		s.writeSanitizedKeys(b.Buffer, s.options.Prefix, job, event)
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
		s.writeSanitizedKeys(b.Buffer, s.options.Prefix, event)
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
		s.writeSanitizedKeys(b.Buffer, s.options.Prefix, job, event)
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
		s.writeSanitizedKeys(b.Buffer, s.options.Prefix, event)
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
			s.options.SanitizationFunc(b, k)
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

func sanitizeKey(b *bytes.Buffer, s string) {
	b.Grow(len(s) + 1)
	for i := 0; i < len(s); i++ {
		si := s[i]
		if ('A' <= si && si <= 'Z') || ('a' <= si && si <= 'z') || ('0' <= si && s[i] <= '9') || si == '_' || si == '.' {
			b.WriteByte(si)
		} else {
			b.WriteByte('$')
		}
	}
}
