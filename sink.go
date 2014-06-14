package health

type CompletionStatus int

const (
	Success CompletionStatus = iota
	ValidationError
	Panic
	Error
	Junk
)

var completionStatusToString = map[CompletionStatus]string{
	Success:         "success",
	ValidationError: "validation_error",
	Panic:           "panic",
	Error:           "error",
	Junk:            "junk",
}

type Sink interface {
	EmitEvent(job string, event string, kvs map[string]string) error
	EmitEventErr(job string, event string, err error, kvs map[string]string) error
	EmitTiming(job string, event string, nanoseconds int64, kvs map[string]string) error
	EmitComplete(job string, status CompletionStatus, nanoseconds int64, kvs map[string]string) error
}
