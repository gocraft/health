package healthd

import (
	"time"
)

// A) don't fire more than every 2 seconds B) the time between an input and output should be at most 2 seconds
func debouncer(doit chan<- struct{}, needitdone <-chan struct{}, threshold time.Duration, sleepTime time.Duration) {
	var oldestNeedItDone time.Time

	for {
		select {
		case <-needitdone:
			if oldestNeedItDone.IsZero() {
				oldestNeedItDone = now()
			}
		default:
			// This sleep time is the max error that we'll be off by.
			time.Sleep(sleepTime)
		}

		if !oldestNeedItDone.IsZero() && (now().Sub(oldestNeedItDone) > threshold) {
			doit <- struct{}{}
			oldestNeedItDone = time.Time{} // Zero the object
		}
	}
}
