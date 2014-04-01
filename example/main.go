package main

import (
	"fmt"
	"health"
	"os"
)




func main() {
	fmt.Println("hi")
	
	// stream := health.NewStream()
	
	sink := health.LogfileWriterSink{os.Stdout}
	sink.EmitEvent("api/v2/tickets/show", "started", nil)
	sink.EmitTiming("api/v2/tickets/show", "started", 13248714, map[string]string{"host": "web33", "request_id": "12345", "subdomain": "slimtimer"})
	
}