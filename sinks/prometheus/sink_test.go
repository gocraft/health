package prometheus

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/gocraft/health"
	"github.com/stretchr/testify/assert"
)

func TestSinkEmitEvent(t *testing.T) {
	config := &Config{
		Endpoint: "http://localhost:9091",
		Job:      "app",
		Hostname: "localhost",
		Interval: time.Second,
	}

	s := NewSink(config)
	defer s.ShutdownServer()

	h := prometheusHandler{
		PayloadChan: make(chan *payload),
	}

	s.EmitEvent("thejob", "theevent", map[string]string{"handler": "/vary/url/path/to/worldnice"})

	go http.ListenAndServe(":9091", h)

	p := <-h.PayloadChan
	assert.Equal(t, p.Url, "/metrics/jobs/app/instances/localhost")
	assert.Contains(t, string(p.Body), "theevent")
	assert.Contains(t, string(p.Body), "thejob")
}

func TestSinkEmitTiming(t *testing.T) {
	config := &Config{
		Endpoint: "http://localhost:9092",
		Job:      "app",
		Hostname: "localhost",
		Interval: time.Second,
	}

	s := NewSink(config)
	defer s.ShutdownServer()

	h := prometheusHandler{
		PayloadChan: make(chan *payload),
	}

	go http.ListenAndServe(":9092", h)

	s.EmitTiming("thetimejob", "theevent", 10, map[string]string{"handler": "/vary/url/path/to/worldnice"})

	p := <-h.PayloadChan
	assert.Equal(t, p.Url, "/metrics/jobs/app/instances/localhost")
	assert.Contains(t, string(p.Body), "theevent")
	assert.Contains(t, string(p.Body), "thetimejob")
	assert.Contains(t, string(p.Body), "handler")
}

func TestSinkEmitComplete(t *testing.T) {
	config := &Config{
		Endpoint: "http://localhost:9093",
		Job:      "app",
		Hostname: "localhost",
		Interval: time.Second,
	}

	s := NewSink(config)
	defer s.ShutdownServer()

	h := prometheusHandler{
		PayloadChan: make(chan *payload),
	}

	go http.ListenAndServe(":9093", h)

	s.EmitComplete("thecomplete", health.Success, 10, map[string]string{"handler": "/oops"})

	p := <-h.PayloadChan
	assert.Equal(t, p.Url, "/metrics/jobs/app/instances/localhost")
	assert.Contains(t, string(p.Body), "thecomplete")
	assert.Contains(t, string(p.Body), "handler")
}

type payload struct {
	Url      string
	Hostname string
	Body     []byte
}

type prometheusHandler struct {
	PayloadChan chan *payload
}

func (h prometheusHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	body, _ := ioutil.ReadAll(r.Body)
	resp := payload{Url: r.URL.Path, Body: body}
	h.PayloadChan <- &resp
	fmt.Fprintf(rw, "OK")
}
