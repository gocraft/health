package bugsnag

import (
	"fmt"
	"github.com/gocraft/health"
	"github.com/gocraft/health/stack"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
	"time"
)

func TestSink(t *testing.T) {
	config := &Config{
		APIKey:       "abcd",
		Endpoint:     "http://localhost:5052/",
		ReleaseStage: "staging",
		AppVersion:   "1.0",
		Hostname:     "",
	}

	s := NewSink(config)
	defer s.ShutdownServer()

	n := notifyHandler{
		PayloadChan: make(chan *payload, 2),
	}

	go http.ListenAndServe(":5052", n)

	err := &health.UnmutedError{Err: fmt.Errorf("err str"), Stack: stack.NewTrace(2)}
	s.EmitEventErr("thejob", "theevent", err, nil)

	p := <-n.PayloadChan
	evt := p.Events[0]
	assert.Equal(t, evt.Context, "thejob")

	ex := evt.Exceptions[0]
	assert.Equal(t, ex.ErrorClass, "theevent")
	assert.Equal(t, ex.Message, "err str")

	err.Emitted = true
	s.EmitEventErr("thejob", "theevent2", err, nil)

	time.Sleep(1 * time.Millisecond)

	select {
	case <-n.PayloadChan:
		t.Errorf("did not expect payload")
	default:
		// yay
	}
}
