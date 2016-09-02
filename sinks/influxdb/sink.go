package influxdb

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/influxdata/influxdb/client/v2"
	"github.com/pubnative/health"
)

type InfluxDBSink struct {
	db        string
	dbhost    string
	hostname  string
	precision string
	notifier  Notifier
	client    *client.Client
	batchA    client.BatchPoints
	batchB    client.BatchPoints
	curBatch  *client.BatchPoints
	swChan    <-chan time.Time
	In        chan *client.Point
}

type Notifier interface {
	Notify(err error)
}

func (s InfluxDBSink) EmitEvent(job string, event string, kvs map[string]string) {
	s.emitPoint(job, kvs, map[string]interface{}{event: 1})
}
func (s InfluxDBSink) EmitEventErr(job string, event string, err error, kvs map[string]string) {
	s.emitPoint(job, kvs, map[string]interface{}{event: 1, "error": 1})
}
func (s InfluxDBSink) EmitTiming(job string, event string, nanoseconds int64, kvs map[string]string) {
	if kvs == nil {
		kvs = make(map[string]string)
	}
	kvs["event"] = event
	timing := map[string]interface{}{"timing": nanoseconds}
	s.emitPoint(job, kvs, timing)
}
func (s InfluxDBSink) EmitGauge(job string, event string, value float64, kvs map[string]string) {
	if kvs == nil {
		kvs = make(map[string]string)
	}
	kvs["event"] = event
	gauge := map[string]interface{}{"gauge": value}
	s.emitPoint(job, kvs, gauge)
}
func (s InfluxDBSink) EmitComplete(job string, status health.CompletionStatus, nanoseconds int64, kvs map[string]string) {
	if kvs == nil {
		kvs = make(map[string]string)
	}
	kvs["status"] = status.String()
	timing := map[string]interface{}{"timing": nanoseconds}
	s.emitPoint(job, kvs, timing)
}

const limit = 1000000

var chanFullErr = errors.New("InfluxDB channel full, dropping point")
var bufferFullErr = errors.New("InfluxDB point buffer overflow, dropping point")

func (s *InfluxDBSink) emitPoint(
	name string,
	tags map[string]string,
	fields map[string]interface{},
) {
	if s.client == nil {
		return
	}
	if tags == nil {
		tags = make(map[string]string)
	}
	tags["hostname"] = s.hostname
	if p, err := client.NewPoint(name, tags, fields, time.Now()); err == nil {
		select {
		case s.In <- p:
		default:
			if s.notifier != nil {
				s.notifier.Notify(chanFullErr)
			}
		}
	} else if s.notifier != nil {
		s.notifier.Notify(err)
	}
}

func (s *InfluxDBSink) sink() {
	for {
		select {
		case <-s.swChan:
			toSend := s.switchBatch()
			go s.send(toSend)
		case p := <-s.In:
			s.bufferPoint(p)
		}
	}
}

func (s *InfluxDBSink) bufferPoint(p *client.Point) {
	if len((*s.curBatch).Points()) >= limit {
		s.notifier.Notify(bufferFullErr)
	} else {
		(*s.curBatch).AddPoint(p)
	}
}

func (s *InfluxDBSink) send(batch *client.BatchPoints) {
	if s.client == nil || len((*batch).Points()) == 0 {
		return
	}
	if _, _, err := (*s.client).Ping(time.Second); err != nil {
		s.connect()
	}
	if err := (*s.client).Write(*batch); err != nil {
		if s.notifier != nil {
			s.notifier.Notify(err)
		}
	}
}

func (s *InfluxDBSink) connect() {
	var conn client.Client
	var err error
	if strings.HasPrefix(s.dbhost, "http") {
		conn, err = client.NewHTTPClient(client.HTTPConfig{Addr: s.dbhost})
	} else {
		conn, err = client.NewUDPClient(client.UDPConfig{Addr: s.dbhost})
		go func() {
			<-time.After(1 * time.Minute)
			s.connect()
		}()
	}
	if err != nil && s.notifier != nil {
		s.notifier.Notify(err)
	}
	if err == nil {
		s.client = &conn
	}
}

func (s *InfluxDBSink) switchBatch() *client.BatchPoints {
	if s.curBatch == &s.batchA {
		s.batchB = s.createBatch()
		s.curBatch = &s.batchB
		return &s.batchA
	} else {
		s.batchA = s.createBatch()
		s.curBatch = &s.batchA
		return &s.batchB
	}
}

func (s *InfluxDBSink) createBatch() client.BatchPoints {
	b, _ := client.NewBatchPoints(client.BatchPointsConfig{
		Database:  s.db,
		Precision: s.precision,
	})
	return b
}

func SetupInfluxDBSink(db, hostname, precision string, notifier Notifier) *InfluxDBSink {
	dbhost := os.Getenv("INFLUXDB_HOST")
	if dbhost == "" {
		return &InfluxDBSink{}
	}

	s := InfluxDBSink{
		hostname:  hostname,
		dbhost:    dbhost,
		db:        db, // note: if using UDP the database is configured by the UDP service
		precision: precision,
		notifier:  notifier,
		swChan:    time.Tick(200 * time.Millisecond),
		In:        make(chan *client.Point, 1000),
	}

	s.connect()
	s.batchA = s.createBatch()
	s.batchB = s.createBatch()
	s.curBatch = &s.batchA

	go s.sink()

	return &s
}
