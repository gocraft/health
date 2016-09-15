package influxdb

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/influxdata/influxdb/client/v2"
	"github.com/pubnative/health"
)

const bufferSize = 10000
const ticker = 200 * time.Millisecond
var chanFullErr = errors.New("InfluxDB channel full, dropping point")

type InfluxDBSink struct {
	db        string
	dbhost    string
	hostname  string
	precision string
	notifier  Notifier
	client    *client.Client
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

func (s *InfluxDBSink) createBatch() client.BatchPoints {
	b, _ := client.NewBatchPoints(client.BatchPointsConfig{
		Database:  s.db,
		Precision: s.precision,
	})
	return b
}

func (s *InfluxDBSink) spawnWorker() {
	w := worker{
		sink: s,
		swTick: time.Tick(ticker),
		batch: s.createBatch(),
	}

	go w.process()
}

type worker struct {
	sink    *InfluxDBSink
	swTick  <-chan time.Time
	batch   client.BatchPoints
}

func (w *worker) process() {
	for {
		select {
		case <-w.swTick:
			w.send()
		case point := <-w.sink.In:
			w.batch.AddPoint(point)
		}
	}
}

func (w *worker) send() {
	toSend := w.batch
	w.batch = w.sink.createBatch()
	go w.sink.send(&toSend)
}


func SetupInfluxDBSink(db, dbHost, hostname, precision string, notifier Notifier, workers int) *InfluxDBSink {
	if dbHost == "" {
		return &InfluxDBSink{}
	}

	s := InfluxDBSink{
		hostname:  hostname,
		dbhost:    dbHost,
		db:        db, // note: if using UDP the database is configured by the UDP service
		precision: precision,
		notifier:  notifier,
		In:        make(chan *client.Point, bufferSize),
	}

	s.connect()

	for i := 0; i < workers; i++ {
		s.spawnWorker()
	}

	return &s
}
