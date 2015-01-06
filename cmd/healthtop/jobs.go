package main

import (
	"encoding/json"
	"fmt"
	"github.com/buger/goterm"
	"github.com/gocraft/health/healthd"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type jobOptions struct {
	Sort string
	Name string
}

func jobsLoop(opts *jobOptions) {
	secondTicker := time.Tick(1 * time.Second)

	var lastApiResponse *healthd.ApiResponseJobs
	var hStatus healthdStatus

	responses := make(chan *healthd.ApiResponseJobs)
	errors := make(chan error)

	go pollHealthDJobs(opts, responses, errors)
	for {
		select {
		case <-secondTicker:
			go pollHealthDJobs(opts, responses, errors)
			printJobs(lastApiResponse, &hStatus)
		case resp := <-responses:
			lastApiResponse = resp
			hStatus.lastSuccessAt = time.Now()
			printJobs(lastApiResponse, &hStatus)
		case err := <-errors:
			hStatus.lastErrorAt = time.Now()
			hStatus.lastError = err
		}
	}
}

func pollHealthDJobs(opts *jobOptions, responses chan *healthd.ApiResponseJobs, errors chan error) {
	var body []byte

	// limit. If name is not set, then limit it to the terminal height.
	// if name IS set, then don't limit it b/c we will currently filter in-memory
	var limit uint
	if opts.Name == "" {
		limit = maxRows()
	}

	values := url.Values{}
	if opts.Sort != "" {
		values.Add("sort", opts.Sort)
	}
	if limit != 0 {
		values.Add("limit", fmt.Sprint(limit))
	}

	uri := "http://" + sourceHostPort + "/healthd/jobs"
	params := values.Encode()
	if params != "" {
		uri = uri + "?" + params
	}

	resp, err := http.Get(uri)
	if err != nil {
		errors <- err
		return
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		errors <- err
		return
	}

	var response healthd.ApiResponseJobs
	if err := json.Unmarshal(body, &response); err != nil {
		errors <- err
		return
	}

	if opts.Name != "" {
		filterJobsByName(&response, opts.Name)
	}

	responses <- &response
}

// Given the api response, remove any job entries that don't have 'name' in them.
func filterJobsByName(resp *healthd.ApiResponseJobs, name string) {
	filteredSlice := []*healthd.Job{}

	for _, job := range resp.Jobs {
		if strings.Contains(job.Name, name) {
			filteredSlice = append(filteredSlice, job)
		}
	}

	resp.Jobs = filteredSlice
}

func printJobs(lastApiResponse *healthd.ApiResponseJobs, status *healthdStatus) {
	goterm.Clear() // Clear current screen
	goterm.MoveCursor(1, 1)
	defer goterm.Flush()
	goterm.Println("Current Time:", status.FmtNow(), "   Status:", status.FmtStatus())

	if lastApiResponse == nil {
		goterm.Println("no data yet")
		return
	}

	columns := []string{
		"Job",
		//		"Jobs/Second", //minute? flag?
		"Total Count",
		"Success",
		"ValidationError",
		"Panic",
		"Error",
		"Junk",
		"Avg Response Time",
		"Stddev",
		"Min",
		"Max",
		"Total",
	}

	for i, s := range columns {
		columns[i] = goterm.Bold(goterm.Color(s, goterm.BLACK))
	}

	table := goterm.NewTable(0, goterm.Width()-1, 5, ' ', 0)
	fmt.Fprintf(table, "%s\n", strings.Join(columns, "\t"))

	for _, job := range lastApiResponse.Jobs {
		printJob(table, job)
	}

	goterm.Println(table)
}

func printJob(table *goterm.Table, job *healthd.Job) {
	fullSuccess := job.Count == job.CountSuccess
	printCellString(job.Name, table, true, false, false)
	printCellInt64(job.Count, table, false, fullSuccess, false)
	printCellInt64(job.CountSuccess, table, fullSuccess, fullSuccess, false)
	printCellInt64(job.CountValidationError, table, job.CountValidationError > 0, false, job.CountValidationError > 0)
	printCellInt64(job.CountPanic, table, job.CountPanic > 0, false, job.CountPanic > 0)
	printCellInt64(job.CountError, table, job.CountError > 0, false, job.CountError > 0)
	printCellInt64(job.CountJunk, table, job.CountJunk > 0, false, job.CountJunk > 0)
	printCellNanos(int64(job.NanosAvg), table, true, false, false)
	printCellNanos(int64(job.NanosStdDev), table, false, false, false)
	printCellNanos(job.NanosMin, table, false, false, false)
	printCellNanos(job.NanosMax, table, false, false, false)
	printCellNanos(job.NanosSum, table, false, false, false)
	fmt.Fprintf(table, "\n")
}

func printCellNanos(nanos int64, table *goterm.Table, isBold, isGreen, isRed bool) {
	var units string
	switch {
	case nanos > 2000000:
		units = "ms"
		nanos /= 1000000
	case nanos > 1000:
		units = "μs"
		nanos /= 1000
	default:
		units = "ns"
	}

	printCellString(fmt.Sprintf("%d %s", nanos, units), table, isBold, isGreen, isRed)
}

func printCellInt64(val int64, table *goterm.Table, isBold, isGreen, isRed bool) {
	printCellString(fmt.Sprint(val), table, isBold, isGreen, isRed)
}

func printCellString(text string, table *goterm.Table, isBold, isGreen, isRed bool) {
	color := goterm.BLACK
	if isGreen {
		color = goterm.GREEN
	} else if isRed {
		color = goterm.RED
	}

	fmt.Fprintf(table, "%s\t", format(text, color, isBold))
}

func format(text string, color int, isBold bool) string {
	if isBold {
		return goterm.Bold(goterm.Color(text, color))
	} else {
		return normal(goterm.Color(text, color))
	}

}

func normal(text string) string {
	return fmt.Sprintf("\033[0m%s\033[0m", text)
}

// Returns the max amount of metrics/rows we can display
// This is the # of rows in the terminal minus 2 (for time / stats + grid header)
// To elimate any weird cases where the terminal is super short, we'll return a min rows of 3
func maxRows() uint {
	n := goterm.Height() - 2
	if n < 3 {
		n = 3
	}
	return uint(n)
}
