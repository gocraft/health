package health

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestJsonPollingSinkServerSuccess(t *testing.T) {
	sink := NewJsonPollingSink(time.Minute, time.Minute*5)
	defer sink.ShutdownServer()

	sink.EmitEvent("myjob", "myevent", nil)
	sink.EmitEventErr("myjob", "myevent", fmt.Errorf("myerr"), nil)
	sink.EmitTiming("myjob", "myevent", 100, nil)
	sink.EmitGauge("myjob", "myevent", 3.14, nil)
	sink.EmitComplete("myjob", Success, 9, nil)

	time.Sleep(10 * time.Millisecond)

	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest("GET", "/health", nil)

	sink.ServeHTTP(recorder, request)

	assert.Equal(t, 200, recorder.Code)

	var resp HealthAggregationsResponse
	err := json.Unmarshal(recorder.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(resp.IntervalAggregations))
	assert.Equal(t, map[string]int64{"myevent": 1}, resp.IntervalAggregations[0].Events)
}

func TestJsonPollingSinkServerNotFound(t *testing.T) {
	sink := NewJsonPollingSink(time.Minute, time.Minute*5)
	defer sink.ShutdownServer()

	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest("GET", "/wat", nil)
	sink.ServeHTTP(recorder, request)
	assert.Equal(t, 404, recorder.Code)
}
