package influxdb

import (
	"testing"
	"time"

	"strings"

	"sort"

	"github.com/gocraft/health"
	"github.com/influxdata/influxdb/client/v2"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestSinkWorker(t *testing.T) {
	client := &testClient{}
	sink := &InfluxDBSink{
		In:     make(chan *point, bufferSize),
		client: client,
	}
	flush := make(chan time.Time)
	w := worker{
		sink:     sink,
		swTick:   flush,
		aggBatch: make(pointsBatch),
	}
	go w.process()

	sink.EmitEvent("abc", "ev1", health.Kvs{"x0^": "y.", "-z": "1xo"})
	sink.EmitEvent("abc", "ev1", health.Kvs{"x0^": "y.", "-z": "1xo"})
	sink.EmitEvent("abc", "ev1", health.Kvs{"x0^": "y.", "-z": "1xo"})
	sink.EmitEvent("abc", "ev2", health.Kvs{"x0^": "y.", "-z": "1xo"})
	sink.EmitEvent("abc", "ev1", health.Kvs{"x0?": "y.", "-z": "1xo"})
	sink.EmitEvent("abc", "ev2", health.Kvs{"x0?": "y.", "-z": "1xo"})
	sink.EmitEvent("abc", "ev1", health.Kvs{"x0^": "y.", "-z": "1ko"})
	sink.EmitEventErr("abc", "ev1", errors.New("!"), health.Kvs{"x0^": "y.", "-z": "1ko"})
	sink.EmitEventErr("abc", "ev1", errors.New("!"), health.Kvs{"x0^": "y.", "-z": "1ko"})

	sink.EmitGauge("abc", "ev1", 1.0, health.Kvs{"x0?": "y.", "-z": "1xo"})
	sink.EmitGauge("abc", "ev1", 1.0, health.Kvs{"x0?": "y.", "-z": "1xo"})

	sink.EmitTiming("abc", "ev1", 12345, health.Kvs{"x0?": "y.", "-z": "1xo"})
	sink.EmitTiming("abc", "ev1", 12345, health.Kvs{"x0?": "y.", "-z": "1xo"})
	time.Sleep(50 * time.Millisecond)

	assert.Len(t, w.batch, 4)
	assert.Len(t, w.aggBatch, 3)

	flush <- time.Now()
	time.Sleep(50 * time.Millisecond)
	pts := client.pointsWritten.Points()

	str := make([]string, len(pts))
	for i := range pts {
		str[i] = strings.TrimRight(pts[i].String(), "0123456789") // remove timestamp
	}
	sort.Strings(str)

	assert.Equal(t, strings.Join(str, ""+
		"\n"),
`abc,-z=1ko,x0^=y. error=2i,ev1=3i
abc,-z=1xo,event=ev1,x0?=y. gauge=1 
abc,-z=1xo,event=ev1,x0?=y. gauge=1 
abc,-z=1xo,event=ev1,x0?=y. timing=12345i 
abc,-z=1xo,event=ev1,x0?=y. timing=12345i 
abc,-z=1xo,x0?=y. ev1=1i,ev2=1i 
abc,-z=1xo,x0^=y. ev1=3i,ev2=1i `)
}

type testClient struct {
	pointsWritten client.BatchPoints
}

func (*testClient) Ping(timeout time.Duration) (time.Duration, string, error) {
	return 0, "", nil
}

func (tc *testClient) Write(bp client.BatchPoints) error {
	tc.pointsWritten = bp
	return nil
}

func (*testClient) Query(q client.Query) (*client.Response, error) {
	panic("implement me")
}

func (*testClient) Close() error {
	panic("implement me")
}
