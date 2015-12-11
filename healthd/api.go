package healthd

import (
	"encoding/json"
	"fmt"
	"github.com/braintree/manners"
	"github.com/gocraft/health"
	"github.com/gocraft/web"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"
)

// Job represents a health.JobAggregation, but designed for JSON-ization without all the nested counters/timers
type Job struct {
	Name                 string `json:"name"`
	Count                int64  `json:"count"`
	CountSuccess         int64  `json:"count_success"`
	CountValidationError int64  `json:"count_validation_error"`
	CountPanic           int64  `json:"count_panic"`
	CountError           int64  `json:"count_error"`
	CountJunk            int64  `json:"count_junk"`

	NanosSum        int64   `json:"nanos_sum"`
	NanosSumSquares float64 `json:"nanos_sum_squares"`
	NanosMin        int64   `json:"nanos_min"`
	NanosMax        int64   `json:"nanos_max"`
	NanosAvg        float64 `json:"nanos_avg"`
	NanosStdDev     float64 `json:"nanos_std_dev"`
}

type apiResponse struct {
	InstanceId       string        `json:"instance_id"`
	IntervalDuration time.Duration `json:"interval_duration"`
}

type ApiResponseJobs struct {
	apiResponse
	Jobs []*Job `json:"jobs"`
}

type ApiResponseAggregations struct {
	apiResponse
	Aggregations []*health.IntervalAggregation `json:"aggregations"`
}

type ApiResponseAggregationsOverall struct {
	apiResponse
	Overall *health.IntervalAggregation `json:"overall"`
}

type ApiResponseHosts struct {
	apiResponse
	Hosts []*HostStatus `json:"hosts"`
}

type apiContext struct {
	hd *HealthD
	*health.Job
}

func (hd *HealthD) apiRouter() http.Handler {
	router := web.New(apiContext{})
	router.NotFound(func(rw web.ResponseWriter, req *web.Request) {
		renderNotFound(rw)
	})

	healthdRouter := router.Subrouter(apiContext{}, "/healthd")

	healthdRouter.Middleware(func(c *apiContext, rw web.ResponseWriter, req *web.Request, next web.NextMiddlewareFunc) {
		c.hd = hd
		next(rw, req)
	})

	healthdRouter.Middleware((*apiContext).SetContentType).
		Middleware((*apiContext).HealthMiddleware).
		Get("/aggregations", (*apiContext).Aggregations).
		Get("/aggregations/overall", (*apiContext).Overall).
		Get("/jobs", (*apiContext).Jobs).
		Get("/hosts", (*apiContext).Hosts)

	return router
}

func (hd *HealthD) startHttpServer(hostPort string, done chan bool) {
	server := manners.NewWithServer(&http.Server{
		Addr:    hostPort,
		Handler: hd.apiRouter(),
	})
	hd.stopHTTP = server.Close
	done <- true
	server.ListenAndServe()
}

func (c *apiContext) SetContentType(rw web.ResponseWriter, req *web.Request, next web.NextMiddlewareFunc) {
	rw.Header().Set("Content-Type", "application/json; charset=utf-8")
	next(rw, req)
}

func (c *apiContext) HealthMiddleware(rw web.ResponseWriter, r *web.Request, next web.NextMiddlewareFunc) {
	c.Job = c.hd.stream.NewJob(r.RoutePath())

	path := r.URL.Path
	c.EventKv("starting_request", health.Kvs{"path": path})

	next(rw, r)

	code := rw.StatusCode()
	kvs := health.Kvs{
		"code": fmt.Sprint(code),
		"path": path,
	}

	// Map HTTP status code to category.
	var status health.CompletionStatus
	// if c.Panic {
	// 	status = health.Panic
	// } else
	if code < 400 {
		status = health.Success
	} else if code == 422 {
		status = health.ValidationError
	} else if code < 500 {
		status = health.Junk // 404, 401
	} else {
		status = health.Error
	}
	c.CompleteKv(status, kvs)
}

func (c *apiContext) Aggregations(rw web.ResponseWriter, r *web.Request) {
	aggregations := c.hd.getAggregationSequence()
	resp := &ApiResponseAggregations{
		apiResponse:  getApiResponse(c.hd.intervalDuration),
		Aggregations: aggregations,
	}
	renderJson(rw, resp)
}

func (c *apiContext) Overall(rw web.ResponseWriter, r *web.Request) {
	aggregations := c.hd.getAggregationSequence()
	overall := combineAggregations(aggregations)
	resp := &ApiResponseAggregationsOverall{
		apiResponse: getApiResponse(c.hd.intervalDuration),
		Overall:     overall,
	}
	renderJson(rw, resp)
}

func (c *apiContext) Jobs(rw web.ResponseWriter, r *web.Request) {
	sort := getSort(r)
	limit := getLimit(r)
	aggregations := c.hd.getAggregationSequence()
	overall := combineAggregations(aggregations)
	jobs := filterJobs(overall, sort, limit)
	resp := &ApiResponseJobs{
		apiResponse: getApiResponse(c.hd.intervalDuration),
		Jobs:        jobs,
	}
	renderJson(rw, resp)
}

func (c *apiContext) Hosts(rw web.ResponseWriter, r *web.Request) {
	hosts := c.hd.getHosts()
	sort.Sort(HostStatusByHostPort(hosts))
	resp := &ApiResponseHosts{
		apiResponse: getApiResponse(c.hd.intervalDuration),
		Hosts:       hosts,
	}
	renderJson(rw, resp)
}

func getApiResponse(duration time.Duration) apiResponse {
	return apiResponse{
		InstanceId:       health.Identifier,
		IntervalDuration: duration,
	}
}

func renderJson(rw http.ResponseWriter, data interface{}) {
	jsonData, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		renderError(rw, err)
		return
	}
	fmt.Fprintf(rw, string(jsonData))
}

func renderNotFound(rw http.ResponseWriter) {
	rw.WriteHeader(404)
	fmt.Fprintf(rw, `{"error": "not_found"}`)
}

func renderError(rw http.ResponseWriter, err error) {
	rw.WriteHeader(500)
	fmt.Fprintf(rw, `{"error": "%s"}`, err.Error())
}

func combineAggregations(aggregations []*health.IntervalAggregation) *health.IntervalAggregation {
	if len(aggregations) == 0 {
		return nil
	}

	overallAgg := health.NewIntervalAggregation(aggregations[0].IntervalStart)
	for _, ia := range aggregations {
		overallAgg.Merge(ia)
	}
	return overallAgg
}

func getSort(r *web.Request) string {
	return r.URL.Query().Get("sort")
}

func getLimit(r *web.Request) int {
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		return 0
	}

	n, err := strconv.ParseInt(limit, 10, 0)
	if err != nil {
		return 0
	}
	return int(n)
}

// By is the type of a "less" function that defines the ordering of its Planet arguments.
type By func(j1, j2 *Job) bool

// Sort is a method on the function type, By, that sorts the argument slice according to the function.
func (by By) Sort(jobs []*Job) {
	js := &jobSorter{
		jobs: jobs,
		by:   by, // The Sort method's receiver is the function (closure) that defines the sort order.
	}
	sort.Sort(js)
}

// planetSorter joins a By function and a slice of Planets to be sorted.
type jobSorter struct {
	jobs []*Job
	by   By
}

// Len is part of sort.Interface.
func (s *jobSorter) Len() int {
	return len(s.jobs)
}

// Swap is part of sort.Interface.
func (s *jobSorter) Swap(i, j int) {
	s.jobs[i], s.jobs[j] = s.jobs[j], s.jobs[i]
}

// Less is part of sort.Interface. It is implemented by calling the "by" closure in the sorter.
func (s *jobSorter) Less(i, j int) bool {
	return s.by(s.jobs[i], s.jobs[j])
}

var jobSorters = map[string]By{
	"name": func(j1, j2 *Job) bool {
		return j1.Name < j2.Name
	},
	"count": func(j1, j2 *Job) bool {
		return j1.Count > j2.Count
	},
	"count_success": func(j1, j2 *Job) bool {
		return j1.CountSuccess > j2.CountSuccess
	},
	"count_validation_error": func(j1, j2 *Job) bool {
		return j1.CountValidationError > j2.CountValidationError
	},
	"count_panic": func(j1, j2 *Job) bool {
		return j1.CountPanic > j2.CountPanic
	},
	"count_error": func(j1, j2 *Job) bool {
		return j1.CountError > j2.CountError
	},
	"count_junk": func(j1, j2 *Job) bool {
		return j1.CountJunk > j2.CountJunk
	},
	"total_time": func(j1, j2 *Job) bool {
		return j1.NanosSum > j2.NanosSum
	},
	"avg": func(j1, j2 *Job) bool {
		return j1.NanosAvg > j2.NanosAvg
	},
	"min": func(j1, j2 *Job) bool {
		return j1.NanosMin > j2.NanosMin
	},
	"max": func(j1, j2 *Job) bool {
		return j1.NanosMax > j2.NanosMax
	},
	"stddev": func(j1, j2 *Job) bool {
		return j1.NanosStdDev > j2.NanosStdDev
	},
}

func sortJobs(jobs []*Job, sort string) {
	if by, ok := jobSorters[sort]; ok {
		by.Sort(jobs)
	}
}

func filterJobs(overall *health.IntervalAggregation, sort string, limit int) []*Job {
	if overall == nil {
		return nil
	}
	jobs := make([]*Job, 0, len(overall.Jobs))

	for k, j := range overall.Jobs {
		var avg, stddev float64
		if j.Count == 0 {
			avg = 0
			stddev = 0
		} else {
			avg = float64(j.NanosSum) / float64(j.Count)
			if j.Count == 1 {
				stddev = 0
			} else {
				num := (float64(j.Count) * j.NanosSumSquares) - math.Pow(float64(j.NanosSum), 2)
				div := float64(j.Count * (j.Count - 1))
				stddev = math.Sqrt(num / div)
			}
		}
		job := &Job{
			Name:                 k,
			Count:                j.Count,
			CountSuccess:         j.CountSuccess,
			CountValidationError: j.CountValidationError,
			CountPanic:           j.CountPanic,
			CountError:           j.CountError,
			CountJunk:            j.CountJunk,
			NanosSum:             j.NanosSum,
			NanosSumSquares:      j.NanosSumSquares,
			NanosMin:             j.NanosMin,
			NanosMax:             j.NanosMax,
			NanosAvg:             avg,
			NanosStdDev:          stddev,
		}
		jobs = append(jobs, job)
	}

	sortJobs(jobs, sort)

	if limit > 0 {
		max := len(jobs)
		if limit > max {
			limit = max
		}
		jobs = jobs[0:limit]
	}

	return jobs
}

type HostStatusByHostPort []*HostStatus

func (a HostStatusByHostPort) Len() int           { return len(a) }
func (a HostStatusByHostPort) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a HostStatusByHostPort) Less(i, j int) bool { return a[i].HostPort < a[j].HostPort }
