package health

import (
	"strings"

	"github.com/gocraft/health/stack"
)

type MutedError struct {
	Err error
}

type UnmutedError struct {
	Err     error
	Stack   *stack.Trace
	Emitted bool
}

func (e *MutedError) Error() string {
	return e.Err.Error()
}

func (e *UnmutedError) Error() string {
	return e.Err.Error()
}

func Mute(err error) *MutedError {
	return &MutedError{Err: err}
}

func wrapErr(err error) error {
	switch err := err.(type) {
	case *MutedError, *UnmutedError:
		return err
	default:
		return &UnmutedError{Err: err, Stack: stack.NewTrace(2)}
	}
}

func muteErrIfContains(err error, s ...string) error {
	errStr := err.Error()
	for _, ignore := range s {
		if strings.Contains(errStr, ignore) {
			return Mute(err)
		}
	}
	return err
}
