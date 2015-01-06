package healthd

import (
	"encoding/json"
	"github.com/gocraft/health"
	"io/ioutil"
	"net/http"
	"time"
)

type pollResponse struct {
	HostPort  string
	Timestamp time.Time

	Err   error
	Code  int
	Nanos int64

	health.HealthAggregationsResponse
}

// poll checks a server
func poll(stream *health.Stream, hostPort string, responses chan<- *pollResponse) {
	job := stream.NewJob("poll")

	var body []byte
	var err error

	response := &pollResponse{
		HostPort:  hostPort,
		Timestamp: now(),
	}

	start := time.Now()

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(metricsUrl(hostPort))
	if err != nil {
		response.Err = job.EventErr("poll.client.get", err)
		goto POLL_FINISH
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)

	response.Nanos = time.Since(start).Nanoseconds() // don't mock b/c we need duration
	response.Code = resp.StatusCode

	if err != nil { // ioutil.ReadAll. We're checking here b/c we still want to capture nanos/code
		response.Err = job.EventErr("poll.ioutil.read_all", err)
		goto POLL_FINISH
	}

	if err := json.Unmarshal(body, &response.HealthAggregationsResponse); err != nil {
		response.Err = job.EventErr("poll.json.unmarshall", err)
		goto POLL_FINISH
	}

POLL_FINISH:

	if response.Err != nil {
		job.CompleteKv(health.Error, health.Kvs{"host_port": hostPort})
	} else {
		job.CompleteKv(health.Success, health.Kvs{"host_port": hostPort})
	}

	responses <- response
}

func metricsUrl(hostPort string) string {
	return "http://" + hostPort + "/health"
}
