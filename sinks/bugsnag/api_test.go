package bugsnag

import (
	"encoding/json"
	"fmt"
	"github.com/gocraft/health/stack"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNotify(t *testing.T) {
	config := &Config{
		APIKey:       "abcd",
		Endpoint:     "http://localhost:5051/",
		ReleaseStage: "staging",
		AppVersion:   "1.0",
		Hostname:     "",
	}

	n := notifyHandler{
		PayloadChan: make(chan *payload, 1),
	}

	go http.ListenAndServe(":5051", n)
	time.Sleep(10 * time.Millisecond)

	err := Notify(config, "users/get", "foo.bar", fmt.Errorf("imanerror"), stack.NewTrace(0), make(map[string]string))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	p := <-n.PayloadChan

	assert.NotNil(t, p)
	assert.Equal(t, p.APIKey, "abcd")
	assert.Equal(t, p.Notifier.Name, "health")
	assert.Equal(t, len(p.Events), 1)

	evt := p.Events[0]
	assert.Equal(t, evt.Context, "users/get")
	assert.Equal(t, evt.App.ReleaseStage, "staging")
	assert.Equal(t, len(evt.Exceptions), 1)

	ex := evt.Exceptions[0]
	assert.Equal(t, ex.ErrorClass, "foo.bar")
	assert.Equal(t, ex.Message, "imanerror")

	frame := ex.Stacktrace[0]
	assert.True(t, strings.HasSuffix(frame.File, "api_test.go"))
	assert.Equal(t, frame.Method, "github.com/gocraft/health/sinks/bugsnag:TestNotify")

}

type notifyHandler struct {
	PayloadChan chan *payload
}

func (h notifyHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Fprintf(rw, "got error in ready body: %v", err)
		return
	}

	var resp payload
	err = json.Unmarshal(body, &resp)
	if err != nil {
		fmt.Fprintf(rw, "got error in unmarshal: %v", err)
		return
	}

	h.PayloadChan <- &resp

	fmt.Fprintf(rw, "OK")
}
