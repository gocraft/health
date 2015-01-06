package main

import (
	"encoding/json"
	"fmt"
	"github.com/buger/goterm"
	"github.com/gocraft/health/healthd"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

func hostsLoop() {
	secondTicker := time.Tick(1 * time.Second)

	var lastApiResponse *healthd.ApiResponseHosts
	var hStatus healthdStatus

	responses := make(chan *healthd.ApiResponseHosts)
	errors := make(chan error)

	go pollHealthDHosts(responses, errors)
	for {
		select {
		case <-secondTicker:
			go pollHealthDHosts(responses, errors)
			printHosts(lastApiResponse, &hStatus)
		case resp := <-responses:
			lastApiResponse = resp
			hStatus.lastSuccessAt = time.Now()
			printHosts(lastApiResponse, &hStatus)
		case err := <-errors:
			hStatus.lastErrorAt = time.Now()
			hStatus.lastError = err
		}
	}
}

func pollHealthDHosts(responses chan *healthd.ApiResponseHosts, errors chan error) {
	var body []byte

	uri := "http://" + sourceHostPort + "/healthd/hosts"

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

	var response healthd.ApiResponseHosts
	if err := json.Unmarshal(body, &response); err != nil {
		errors <- err
		return
	}

	responses <- &response
}

func printHosts(lastApiResponse *healthd.ApiResponseHosts, status *healthdStatus) {
	goterm.Clear() // Clear current screen
	goterm.MoveCursor(1, 1)
	defer goterm.Flush()
	goterm.Println("Current Time:", status.FmtNow(), "   Status:", status.FmtStatus())

	//
	if lastApiResponse == nil {
		goterm.Println("no data yet")
		return
	}

	columns := []string{
		"Host:Port",
		"Status",
		"Last Checked",
		"Last Response Time",
	}

	for i, s := range columns {
		columns[i] = goterm.Bold(goterm.Color(s, goterm.BLACK))
	}

	table := goterm.NewTable(0, goterm.Width()-1, 5, ' ', 0)
	fmt.Fprintf(table, "%s\n", strings.Join(columns, "\t"))

	for _, host := range lastApiResponse.Hosts {
		printHost(table, host)
	}

	goterm.Println(table)
}

func printHost(table *goterm.Table, host *healthd.HostStatus) {
	success := host.LastCode == 200 && host.LastErr == ""
	var status string
	if success {
		status = "Success"
	} else if host.LastCheckTime.IsZero() {
		status = "Unknown"
	} else {
		status = "Failure: " + host.LastErr
	}

	printCellString(host.HostPort, table, true, false, false)
	printCellString(status, table, false, success, !success)
	printCellString(host.LastCheckTime.Format(time.RFC1123), table, false, false, false)
	printCellNanos(int64(host.LastNanos), table, false, false, false)
	fmt.Fprintf(table, "\n")
}
