package main

import (
	"fmt"
	"health"
	"os"
	"dbr"
)




func main() {
	fmt.Println("hi")
	
	// stream := health.NewStream()
	
	// sink := health.LogfileWriterSink{os.Stdout}
	// sink.EmitEvent("api/v2/tickets/show", "started", nil)
	// sink.EmitTiming("api/v2/tickets/show", "started", 13248714, map[string]string{"host": "web33", "request_id": "12345", "subdomain": "slimtimer"})
	
	stream := health.NewStream()
	
	stream.KeyValue("host", "web33")
	
	stream.AddLogfileWriterSink(os.Stdout)
	
	job := stream.Job("api/v1/tickets/create")
	
	job.KeyValue("something", "else")
	
	job.Event("poopin")
	job.EventKv("poopin", health.Kvs{"wat": "ok", "etc": "yeah"})
	
	dbr.DoSomething(job)
	
}