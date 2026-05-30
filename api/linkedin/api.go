package linkedin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"jobSearching/db"
	"jobSearching/models"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/imroc/req/v3"
	"github.com/jackc/pgx/v5/pgxpool"
)

var httpClient = func() *req.Client {
	c := req.NewClient()
	// Register shuffle before ImpersonateChrome so it sits innermost in the transport
	// chain — it runs after SetCommonHeaderOrder injects Chrome's fixed order, letting
	// us randomise it before the request is actually sent.
	c.GetTransport().WrapRoundTripFunc(shuffleHeaderOrder)
	c.ImpersonateChrome().
		SetCommonHeader("Accept-Language", "en-US,en;q=0.9").
		SetCookieJar(nil)
	return c
}()

// shuffleHeaderOrder randomises the header send-order on every request.
func shuffleHeaderOrder(rt http.RoundTripper) req.HttpRoundTripFunc {
	return func(r *http.Request) (*http.Response, error) {
		if order := r.Header[req.HeaderOderKey]; len(order) > 0 {
			shuffled := make([]string, len(order))
			copy(shuffled, order)
			rand.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
			r.Header[req.HeaderOderKey] = shuffled
		}
		return rt.RoundTrip(r)
	}
}

// Configure applies environment-driven settings to the HTTP client.
// Must be called after environment variables are loaded.
func Configure() error {
	proxyURL := os.Getenv("DECODO_PROXY_URL")
	if proxyURL == "" {
		return fmt.Errorf("DECODO_PROXY_URL is not set")
	}
	httpClient.SetProxyURL(proxyURL)
	if u, err := url.Parse(proxyURL); err == nil {
		log.Printf("using Decodo proxy: %s", u.Host)
	} else {
		log.Printf("using Decodo proxy")
	}
	return nil
}

type Keywords string

const (
	KeywordsSoftwareEngineer Keywords = "software+engineer"
	KeywordsDataEngineer     Keywords = "data+engineer"
)

type TimeFilter string

const (
	OneDay  TimeFilter = "r86400"
	OneHr   TimeFilter = "r3600"
	OneWeek TimeFilter = "r604800"
)

var workTypeValues = map[models.WorkType]int{
	models.Remote: 2,
	models.Onsite: 1,
	models.Hybrid: 3,
}

type JobType string

const (
	FullTime  JobType = "F"
	PartTime  JobType = "P"
	Contract  JobType = "C"
	Temporary JobType = "T"
)

type SearchOptions struct {
	Keywords   Keywords
	TimePosted TimeFilter
	WorkType   models.WorkType
	JobType    JobType
	Start      int
}

//location=New+York
//locationId=         # LinkedIn's internal geo ID
//geoId=              # alternative geo identifier
//f_TPR=r86400        # time posted: r3600=1hr, r86400=24hr, r604800=week
//f_WT=2              # work type: 1=onsite, 2=remote, 3=hybrid
//f_E=2,3             # experience: 1=internship, 2=entry, 3=associate, 4=mid, 5=director, 6=executive
//f_JT=F,P,C          # job type: F=fulltime, P=parttime, C=contract, T=temporary
//start=0             # pagination offset, increments of 10

const jobPostingBaseURL = "https://www.linkedin.com/jobs-guest/jobs/api/jobPosting/"

// getUrl applies a random delay then performs a GET request, returning the underlying *http.Response.
func getUrl(ctx context.Context, url string) (*http.Response, error) {
	randomDelay()
	resp, err := httpClient.R().SetContext(ctx).Get(url)
	if err != nil {
		return nil, err
	}
	return resp.Response, nil
}

// buildSearchURL constructs the full LinkedIn job search URL for the given options,
// including all query parameters. Used by SearchJobs and for request logging.
func buildSearchURL(opts SearchOptions) string {
	base := "https://www.linkedin.com/jobs-guest/jobs/api/seeMoreJobPostings/search"
	params := url.Values{}
	params.Set("keywords", string(opts.Keywords))
	params.Set("location", "United+States")
	params.Set("geoId", "103644278")
	params.Set("f_TPR", string(opts.TimePosted))
	params.Set("f_WT", strconv.Itoa(workTypeValues[opts.WorkType]))
	params.Set("start", strconv.Itoa(opts.Start))
	if opts.JobType != "" {
		params.Set("f_JT", string(opts.JobType))
	}
	return base + "?" + params.Encode()
}

// SearchJobs fetches a page of LinkedIn job listings matching the given options.
// The Start field in opts controls the pagination offset.
func SearchJobs(ctx context.Context, searchOptions SearchOptions) (*http.Response, error) {
	if err := searchOptions.Validate(); err != nil {
		return nil, err
	}
	getURL := buildSearchURL(searchOptions)
	log.Printf("GET %s", getURL)
	return getUrl(ctx, getURL)
}

// Validate returns an error if any required SearchOptions field is missing.
func (searchOptions SearchOptions) Validate() error {
	if searchOptions.Keywords == "" {
		return fmt.Errorf("Keywords is required")
	}
	if searchOptions.TimePosted == "" {
		return fmt.Errorf("TimePosted is required")
	}
	if searchOptions.WorkType == "" {
		return fmt.Errorf("WorkType is required")
	}
	return nil
}

// SearchJobId fetches the detail page HTML for a single LinkedIn job posting.
func SearchJobId(ctx context.Context, jobId string) (*http.Response, error) {
	return getUrl(ctx, jobPostingBaseURL+jobId)
}

// headersToJSON serialises an http.Header map into a compact JSON object string for logging.
func headersToJSON(h http.Header) string {
	m := make(map[string]string, len(h))
	for k, vals := range h {
		m[k] = strings.Join(vals, ", ")
	}
	b, _ := json.Marshal(m)
	return string(b)
}

const (
	numWorkers      = 5
	minDelay        = 3 * time.Second
	maxJitter       = 7 * time.Second
	macroBreakEvery = 200
	macroBreakMin   = time.Minute
	macroBreakMax   = 2 * time.Minute
)

var (
	requestCount atomic.Int64
	delayMu      sync.Mutex
)

// randomDelay serialises all outbound requests behind a mutex so they fire
// one at a time with a random gap. Every macroBreakEvery calls it holds the
// lock for a longer macro-break, which blocks all workers until it completes.
func randomDelay() {
	n := requestCount.Add(1)
	delayMu.Lock()
	defer delayMu.Unlock()
	if n%macroBreakEvery == 0 {
		pause := macroBreakMin + time.Duration(rand.Int63n(int64(macroBreakMax-macroBreakMin)))
		log.Printf("macro-break: pausing %s after %d requests", pause.Round(time.Second), n)
		time.Sleep(pause)
		return
	}
	time.Sleep(minDelay + time.Duration(rand.Int63n(int64(maxJitter))))
}

// ScrapeJobs paginates through up to numberOfJobs listings, fetches detail pages for new ones,
// and records a scrape run summary on completion.
func ScrapeJobs(ctx context.Context, numberOfJobs int, opts SearchOptions, pool *pgxpool.Pool) {
	startedAt := time.Now()
	runId := uuid.New().String()

	var (
		saved   atomic.Int64
		skipped atomic.Int64
		found   atomic.Int64
	)
	processAllJobs(ctx, pool, opts, numberOfJobs, &found, runId)
	retryFailedJobs(ctx, pool, opts, &found, runId)
	processAllJobDetails(ctx, pool, opts.WorkType, runId, &saved, &skipped)
	retryFailedJobDetails(ctx, pool, opts.WorkType, runId, &saved, &skipped)
	processApplicantUpdates(ctx, pool, opts.WorkType, runId)

	run := models.ScrapeRun{
		RunID:       runId,
		Source:      models.LinkedIn,
		Keywords:    string(opts.Keywords),
		TimePosted:  string(opts.TimePosted),
		WorkType:    string(opts.WorkType),
		JobType:     string(opts.JobType),
		StartedAt:   startedAt,
		FinishedAt:  time.Now(),
		JobsFound:   found.Load(),
		JobsNew:     saved.Load(),
		JobsSkipped: skipped.Load(),
	}

	if err := db.InsertScrapeRun(ctx, pool, run); err != nil {
		log.Printf("failed to log scrape run: %v", err)
	}

	log.Printf("scrape complete: %d found, %d new, %d skipped", run.JobsFound, run.JobsNew, run.JobsSkipped)
}

// processAllJobDetails fetches all new jobs for the run from the DB and concurrently
// retrieves their detail pages, saving results and updating saved/skipped counts.
func processAllJobDetails(
	ctx context.Context,
	pool *pgxpool.Pool,
	workType models.WorkType,
	runId string,
	saved *atomic.Int64,
	skipped *atomic.Int64) {
	jobs, err := db.GetNewJobsByRunID(ctx, pool, runId)
	if err != nil {
		log.Printf("failed to get jobs for run %s: %v", runId, err)
		return
	}

	wg := sync.WaitGroup{}
	sem := make(chan struct{}, numWorkers)
	for _, job := range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := processJobDetail(ctx, pool, job, workType); err != nil {
				log.Printf("failed to process job detail %s: %v", job.SourceID, err)
				skipped.Add(1)
				return
			}
			saved.Add(1)
			log.Printf("saved: %s", job.SourceID)
		}()
	}
	wg.Wait()
}

// retryFailedJobDetails looks up jobs from the run that had is_issue=true request log entries
// and still lack a detail record, then retries them. On success it moves the count from
// skipped to saved; on continued failure the job stays counted as skipped.
func retryFailedJobDetails(
	ctx context.Context,
	pool *pgxpool.Pool,
	workType models.WorkType,
	runId string,
	saved *atomic.Int64,
	skipped *atomic.Int64,
) {
	jobs, err := db.GetFailedJobsByRunID(ctx, pool, runId)
	if err != nil {
		log.Printf("failed to get failed jobs for retry: %v", err)
		return
	}
	if len(jobs) == 0 {
		return
	}
	log.Printf("retrying %d failed job details", len(jobs))

	wg := sync.WaitGroup{}
	sem := make(chan struct{}, numWorkers)
	for _, job := range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := processJobDetail(ctx, pool, job, workType); err != nil {
				log.Printf("retry failed for job detail %s: %v", job.SourceID, err)
				return
			}
			skipped.Add(-1)
			saved.Add(1)
			log.Printf("retry saved: %s", job.SourceID)
		}()
	}
	wg.Wait()
}

// processApplicantUpdates re-fetches detail pages for jobs from the current run that already
// have a detail record but still have an applicant count below the 200-applicant cap.
func processApplicantUpdates(ctx context.Context, pool *pgxpool.Pool, workType models.WorkType, runId string) {
	jobs, err := db.GetJobsForApplicantUpdateByRunID(ctx, pool, runId)
	if err != nil {
		log.Printf("failed to get jobs for applicant update: %v", err)
		return
	}
	if len(jobs) == 0 {
		return
	}
	log.Printf("updating applicants for %d jobs", len(jobs))

	wg := sync.WaitGroup{}
	sem := make(chan struct{}, numWorkers)
	for _, job := range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := processJobDetail(ctx, pool, job, workType); err != nil {
				log.Printf("failed to update applicants for job %s: %v", job.SourceID, err)
			}
		}()
	}
	wg.Wait()
}

// processAllJobs concurrently fetches all search result pages up to numberOfJobs
// and upserts each discovered listing into the database tagged with runId.
func processAllJobs(ctx context.Context,
	pool *pgxpool.Pool,
	opts SearchOptions, numberOfJobs int, found *atomic.Int64, runId string) {
	wg := sync.WaitGroup{}
	sem := make(chan struct{}, numWorkers)
	for i := opts.Start; i < (opts.Start + numberOfJobs); i += 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			searchOptions := opts
			searchOptions.Start = i
			_ = processJob(ctx, pool, searchOptions, found, runId)
		}()
	}
	wg.Wait()
}

// retryFailedJobs looks up search pages from the run that had is_issue=true request log entries
// and retries them. The start offset is parsed from the stored URL.
func retryFailedJobs(ctx context.Context, pool *pgxpool.Pool, opts SearchOptions, found *atomic.Int64, runId string) {
	urls, err := db.GetFailedSearchURLsByRunID(ctx, pool, runId)
	if err != nil {
		log.Printf("failed to get failed search pages for retry: %v", err)
		return
	}
	if len(urls) == 0 {
		return
	}
	log.Printf("retrying %d failed job pages", len(urls))

	wg := sync.WaitGroup{}
	sem := make(chan struct{}, numWorkers)
	for _, rawURL := range urls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			u, err := url.Parse(rawURL)
			if err != nil {
				log.Printf("retry: could not parse search URL %q: %v", rawURL, err)
				return
			}
			offset, err := strconv.Atoi(u.Query().Get("start"))
			if err != nil {
				log.Printf("retry: could not parse start offset from %q: %v", rawURL, err)
				return
			}
			searchOptions := opts
			searchOptions.Start = offset
			if err := processJob(ctx, pool, searchOptions, found, runId); err != nil {
				log.Printf("retry failed for job page at start=%d: %v", offset, err)
			}
		}()
	}
	wg.Wait()
}

// processJob fetches one search result page, parses the listings, and upserts each
// into the database. found is incremented by the number of jobs returned by LinkedIn.
func processJob(
	ctx context.Context, pool *pgxpool.Pool,
	searchOptions SearchOptions, found *atomic.Int64, runId string) error {
	searchURL := buildSearchURL(searchOptions)
	baseLog := models.RequestLog{
		Source:         models.LinkedIn,
		URL:            searchURL,
		RequestHeaders: "{}",
		RunID:          runId,
	}

	resp, err := SearchJobs(ctx, searchOptions)
	if err != nil {
		entry := baseLog
		entry.Error = err.Error()
		entry.Message = "network error fetching search page"
		entry.IsIssue = true
		db.InsertRequestLog(ctx, pool, entry)
		return err
	}

	statusCode := resp.StatusCode
	baseLog.StatusCode = &statusCode
	if resp.Request != nil {
		baseLog.RequestHeaders = headersToJSON(resp.Request.Header)
	}

	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		entry := baseLog
		entry.Error = err.Error()
		entry.Message = "failed to read search page body"
		entry.IsIssue = true
		db.InsertRequestLog(ctx, pool, entry)
		return err
	}
	bodyStr := string(body)

	jobs, err := ParseJobs(bodyStr, models.LinkedIn)
	if err != nil {
		entry := baseLog
		entry.Error = err.Error()
		entry.Message = "failed to parse search page"
		entry.ResponseBody = bodyStr
		entry.IsIssue = true
		db.InsertRequestLog(ctx, pool, entry)
		return err
	}

	if len(jobs) == 0 {
		entry := baseLog
		entry.Message = "empty response from search page"
		entry.ResponseBody = bodyStr
		entry.IsIssue = true
		db.InsertRequestLog(ctx, pool, entry)
		log.Printf("empty response at start=%d", searchOptions.Start)
		return nil
	}

	entry := baseLog
	entry.Message = "ok"
	db.InsertRequestLog(ctx, pool, entry)

	found.Add(int64(len(jobs)))
	log.Printf("page start=%d: found %d jobs", searchOptions.Start, len(jobs))

	for _, job := range jobs {
		job.RunID = runId
		if _, _, err := db.UpsertJob(ctx, pool, job); err != nil {
			log.Printf("failed to upsert job %s: %v", job.SourceID, err)
		}
	}

	return nil
}

// processJobDetail fetches and saves the detail page for a job that was discovered in the
// current run. Returns nil on success or a non-nil error on failure.
func processJobDetail(ctx context.Context, pool *pgxpool.Pool, job models.Job, workType models.WorkType) error {

	baseLog := models.RequestLog{
		Source:         models.LinkedIn,
		JobSourceID:    job.SourceID,
		URL:            jobPostingBaseURL + job.SourceID,
		RequestHeaders: "{}",
		RunID:          job.RunID,
	}

	resp, err := SearchJobId(ctx, job.SourceID)
	if err != nil {
		entry := baseLog
		entry.Error = err.Error()
		entry.Message = "network error fetching job detail"
		entry.IsIssue = true
		db.InsertRequestLog(ctx, pool, entry)
		return fmt.Errorf("fetch detail: %w", err)
	}

	statusCode := resp.StatusCode
	baseLog.RequestHeaders = headersToJSON(resp.Request.Header)
	baseLog.StatusCode = &statusCode

	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		entry := baseLog
		entry.Error = err.Error()
		entry.Message = "failed to read response body"
		entry.IsIssue = true
		db.InsertRequestLog(ctx, pool, entry)
		return fmt.Errorf("read body: %w", err)
	}
	bodyStr := string(body)

	if statusCode != http.StatusOK {
		entry := baseLog
		entry.Message = fmt.Sprintf("non-200 status code: %d", statusCode)
		entry.ResponseBody = bodyStr
		entry.IsIssue = true
		db.InsertRequestLog(ctx, pool, entry)
		return fmt.Errorf("unexpected status %d for job %s", statusCode, job.SourceID)
	}

	detail, err := ParseJobDetail(bodyStr)
	if err != nil {
		entry := baseLog
		entry.Error = err.Error()
		entry.Message = "failed to parse job detail HTML"
		entry.ResponseBody = bodyStr
		entry.IsIssue = true
		db.InsertRequestLog(ctx, pool, entry)
		return fmt.Errorf("parse detail: %w", err)
	}

	detail.WorkType = string(workType)

	entry := baseLog
	if detail.Description == "" {
		log.Printf("empty description for job %s", job.SourceID)
		entry.Message = "parse succeeded but description was empty"
		entry.ResponseBody = bodyStr
		entry.IsIssue = true
	} else {
		entry.Message = "ok"
	}
	db.InsertRequestLog(ctx, pool, entry)

	if err := db.InsertJobDetail(ctx, pool, job, detail); err != nil {
		return fmt.Errorf("insert detail: %w", err)
	}

	return nil
}
