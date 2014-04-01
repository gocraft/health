package health

import (
	"fmt"
)

type Stream struct {
	
}


func foo() {
	
	// At app initialization:
	stream := health.New(os.Stdout)
	stream.KeyValue("host", "web33") // You can do global K/V
	
	// But to actually emit anything (log, metrics, etc), you need to be in a system
	
	
	// If you know your system right away:
	system := stream.System("api/v2/tickets/create")
	
	
	// If you need to do some shit before-hand (before you know which system it is)
	// You should attribute it to a before filter system:
	system := stream.System("authentication")
	
	// and later on, switch the system:
	system := stream.System("api/v2/tickets/create")
	
	
	
	// OOOOK, once you identify your system, you can log stuff:
	// NOTE: this is not for sentence level shit. Logs are for computers.
	// NOTE2: This will also increment the counter something_happened
	system.Log("something_happend", health.Kv{"field": "value"})
	
	// You can set k/v fields on the system. All future log entries will contain these k/v entries. Logs before it will not.
	system.KeyValue("subdomain_id", "123")
	
	
	// We can also do counters and timers:
	system.Incr("happenings")
	system.Incr("happenings/specific_happening")
	// NOTE: we could get away with not doing this
	
	// When you're finally done with a system:
	
	system.Success() // Does a timer on the system + a success counter
	system.UnhandledError()   // timer + error counter
	system.ValidationError()   // timer + validation counter
	
	// Both UnhandledError and ValidationError are Log statements: .ValidationError("tickets/title/too_long", Kv{"length": 1230})
	
	
	// mysql, redis, memcached, etc might need to write to metrics/log entries during your system:
	
	mysqlSession.SetSystem(system)
	
	// generally 2 ways to do it:
	// dbr could import Health:
	//   mysqlSession.SetSystem(system)
	// or
	// dbr could expose callbacks, which we'd hook into:
	//   
	
}