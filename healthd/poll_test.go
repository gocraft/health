package healthd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/braintree/manners"
	"github.com/gocraft/health"
	"github.com/stretchr/testify/assert"
)

func TestPoll(t *testing.T) {
	setNowMock("2011-09-09T23:36:13Z")
	defer resetNowMock()

	intAgg := health.NewIntervalAggregation(now())
	data := &health.HealthAggregationsResponse{
		InstanceId:           "web22.12345",
		IntervalDuration:     time.Minute,
		IntervalAggregations: []*health.IntervalAggregation{intAgg},
	}
	stop := serveJson(":5050", data)
	defer func() {
		stop()
	}()

	responses := make(chan *pollResponse, 2)
	poll(health.NewStream(), ":5050", responses)
	response := <-responses

	assert.NotNil(t, response)
	assert.Equal(t, response.HostPort, ":5050")
	assert.Equal(t, response.Timestamp, now())
	assert.Nil(t, response.Err)
	assert.Equal(t, response.Code, 200)
	assert.True(t, response.Nanos > 0 && response.Nanos < int64(time.Second))
	assert.Equal(t, response.InstanceId, "web22.12345")
	// we'll just "trust" that the other stuff gets unmarshalled correctly. We didn't really put anything in there anyway in this test.
}

// serveJson will start a server on the hostPort and serve any path the Jsonified data.
// Each successive HTTP request will return the next data.
// If there is only one data, it will be returned on each request.
func serveJson(hostPort string, data ...interface{}) func() bool {
	var curData = 0

	var f http.HandlerFunc
	f = func(rw http.ResponseWriter, r *http.Request) {
		d := data[curData]
		curData = (curData + 1) % len(data)
		jsonData, err := json.MarshalIndent(d, "", "\t")
		if err != nil {
			panic(err)
		}
		fmt.Fprintf(rw, string(jsonData))
	}

	go manners.ListenAndServe(hostPort, f)
	time.Sleep(10 * time.Millisecond)

	return manners.Close
}
