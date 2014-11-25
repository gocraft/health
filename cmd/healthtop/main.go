package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"time"
)

// v2:
// jobs/min vs jobs total vs jobs/sec (eg normalization)
// - errors ??????
// - tests
// - switch views/sorts while inside healthd

type healthdStatus struct {
	lastSuccessAt time.Time
	lastErrorAt   time.Time
	lastError     error
}

func (s *healthdStatus) FmtNow() string {
	return time.Now().Format(time.RFC1123)
}

func (s *healthdStatus) FmtStatus() string {
	if s.lastErrorAt.IsZero() && s.lastSuccessAt.IsZero() {
		return "[starting...]"
	} else if s.lastErrorAt.After(s.lastSuccessAt) {
		return fmt.Sprint("[error: '", s.lastError.Error(), "'    LastErrorAt: ", s.lastErrorAt.Format(time.RFC1123), "]")
	} else {
		return "[success]"
	}
}

var sourceHostPort string

func main() {
	var cmdRoot = &cobra.Command{
		Use: "healthtop [command]",
	}
	cmdRoot.PersistentFlags().StringVar(&sourceHostPort, "source", "localhost:5032", "source is the host:port of the healthd to query. ex: localhost:5031")

	var sort string
	var name string

	var cmdJobs = &cobra.Command{
		Use:   "jobs",
		Short: "list jobs",
		Run: func(cmd *cobra.Command, args []string) {
			jobsLoop(&jobOptions{Name: name, Sort: sort})
		},
	}

	cmdJobs.Flags().StringVar(&sort, "sort", "name", "sort ∈ {name, count, count_success, count_XXX, min, max, avg}")
	cmdJobs.Flags().StringVar(&name, "name", "", "name is a partial match on the name")

	var cmdHosts = &cobra.Command{
		Use:   "hosts",
		Short: "list hosts",
		Run: func(cmd *cobra.Command, args []string) {
			hostsLoop()
		},
	}

	cmdRoot.AddCommand(cmdJobs)
	cmdRoot.AddCommand(cmdHosts)
	cmdRoot.Execute()
}
