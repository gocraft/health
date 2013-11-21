


Health is a package that offers hierarchical logging


You can have multiple root logstreams.

Streams can have substreams.

Given a stream, you can get a log.Logger that writes to it.

Usage:

// Make new streams:
stream := health.New(os.Stdout, "general")

// You can create substreams with tags:
gorpStream := stream.Tag("gorp")

// Then, pass that stubstream to gorp:
gorp.TraceOn("", gorpStream.Logger())

// You can log directly to streams (probably with the same methods as Logger)
stream.Println("etc", myObject)
stream.Printf("etc %d", 3)

// Error vs Info can be handled by different streams
stream := health.New(os.Stdout, "general") // Root stream
errorStream := stream.Tag("error")
infoStream := stream.Tag("info")
warningStream := stream.Tag("warning")

// You can do k/v properties on streams:
stream := health.New(os.Stdout, "general").KeyValue("host", os.Hostname())
                                          .KeyValue("pid", os.Pid())



// When logging, you can append extra adhoc k/v to log entries:
// (eg, making substreams should be cheap)
stream.KeyValue("user_id", user.Id).Println("This user omg'ed")

// Given a stream, you can format its output:
stream.Format("TODO WAT")

// You can subscribe to a stream:
stream.subscribe(func (message string, props map[string]string) { fmt.Println("omg i got ", message) })

// You can subscribe to a substream:
stream.Tag("gorp").subscribe(...)
