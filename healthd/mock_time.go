package healthd

import (
	"time"
)

var nowMock time.Time

func now() time.Time {
	if nowMock.IsZero() {
		return time.Now()
	}
	return nowMock
}

func setNowMock(t string) {
	var err error
	nowMock, err = time.Parse(time.RFC3339, t)
	if err != nil {
		panic(err)
	}
}

func advanceNowMock(dur time.Duration) {
	if nowMock.IsZero() {
		panic("nowMock is not set")
	}
	nowMock = nowMock.Add(dur)
}

func resetNowMock() {
	nowMock = time.Time{}
}
