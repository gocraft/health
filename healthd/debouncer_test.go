package healthd

import (
	"testing"
	"time"
)

func TestDebouncer(t *testing.T) {
	doit := make(chan struct{})
	needitdone := make(chan struct{})

	setNowMock("2011-09-09T23:36:13Z")
	defer resetNowMock()

	go debouncer(doit, needitdone, time.Second*2, time.Millisecond)

	needitdone <- struct{}{}
	needitdone <- struct{}{}

	time.Sleep(time.Millisecond * 2)

	select {
	case <-doit:
		t.Error("Did it too soon")
	default:
		// cool
	}

	advanceNowMock(time.Second * 1)
	time.Sleep(time.Millisecond * 2) // Need the goroutine to wake up

	select {
	case <-doit:
		t.Error("Did it too soon")
	default:
		// cool
	}

	advanceNowMock(time.Second * 2)
	time.Sleep(time.Millisecond * 2) // Need the goroutine to wake up

	select {
	case <-doit:
		// cool
	default:
		t.Error("never did it")
	}

	time.Sleep(time.Millisecond * 2) // Need the goroutine to wake up

	select {
	case <-doit:
		t.Error("should only do it once")
	default:
		// cool
	}
}
