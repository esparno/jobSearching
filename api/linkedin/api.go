package linkedin

import (
	"context"
	"fmt"
	"io"
	"jobSearching/db"
	"jobSearching/models"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var httpClient = &http.Client{}

type Keywords string

const (
	KeywordsSoftwareEngineer Keywords = "software+engineer"
	KeywordsDataEngineer     Keywords = "data+engineer"
)

type timePostedLookup string

const (
	OneDay  timePostedLookup = "r86400"
	OneHr   timePostedLookup = "r3600"
	OneWeek timePostedLookup = "r604800"
)

var workTypeValues = map[models.WorkType]int{
	models.Remote: 2,
	models.Onsite: 1,
	models.Hybrid: 3,
}

type jobType string

const (
	FullTime  jobType = "F"
	PartTime  jobType = "P"
	Contract  jobType = "C"
	Temporary jobType = "T"
)

type SearchOptions struct {
	Keywords   Keywords
	TimePosted timePostedLookup
	WorkType   models.WorkType
	JobType    jobType
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

func SearchJobs(searchOptions SearchOptions) (*http.Response, error) {
	if err := searchOptions.Validate(); err != nil {
		return nil, err
	}

	searchURL := "https://www.linkedin.com/jobs-guest/jobs/api/seeMoreJobPostings/search"

	params := url.Values{}
	params.Set("keywords", string(searchOptions.Keywords))
	params.Set("location", "United+States")
	params.Set("geoId", "103644278")
	params.Set("f_TPR", string(searchOptions.TimePosted))
	params.Set("f_WT", strconv.Itoa(workTypeValues[searchOptions.WorkType]))
	params.Set("start", strconv.Itoa(searchOptions.Start))

	if searchOptions.JobType != "" {
		params.Set("f_JT", string(searchOptions.JobType))
	}

	getURL := searchURL + "?" + params.Encode()
	log.Printf("GET %s", getURL)
	return httpClient.Get(getURL)
}

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

func SearchJobId(jobId string) (*http.Response, error) {
	return httpClient.Get(jobPostingBaseURL + jobId)
}

func headersToJSON(h http.Header) string {
	b := strings.Builder{}
	b.WriteString("{")
	first := true
	for k, vals := range h {
		if !first {
			b.WriteString(",")
		}
		_, _ = fmt.Fprintf(&b, "%q:%q", k, strings.Join(vals, ", "))
		first = false
	}
	b.WriteString("}")
	return b.String()
}

const (
	numWorkers = 1
	minDelay   = 2 * time.Second
	maxJitter  = 3 * time.Second
)

func randomDelay() {
	time.Sleep(minDelay + time.Duration(rand.Int63n(int64(maxJitter))))
}

func ScrapeJobs(ctx context.Context, numberOfJobs int, opts SearchOptions, pool *pgxpool.Pool) {
	startedAt := time.Now()
	jobsCh := make(chan models.Job, (numberOfJobs-opts.Start)/10)

	var found atomic.Int64
	go paginate(opts, numberOfJobs, jobsCh, &found)

	var (
		wg      sync.WaitGroup
		saved   atomic.Int64
		skipped atomic.Int64
	)
	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobsCh {
				isNew, err := processJob(ctx, pool, job, opts.WorkType)
				if err != nil {
					log.Printf("failed to process job %s: %v", job.SourceID, err)
					continue
				}
				if isNew {
					saved.Add(1)
					log.Printf("saved: %s | %s | %s", job.SourceID, job.Title, job.Company)
					randomDelay()
				} else {
					skipped.Add(1)
					log.Printf("skipped: %s | %s | %s", job.SourceID, job.Title, job.Company)
				}
			}
		}()
	}

	wg.Wait()

	run := models.ScrapeRun{
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

func paginate(opts SearchOptions, numberOfJobs int, jobsCh chan<- models.Job, found *atomic.Int64) {
	defer close(jobsCh)
	for i := opts.Start; i < numberOfJobs; i += 10 {
		searchOptions := opts
		searchOptions.Start = i

		resp, err := SearchJobs(searchOptions)
		if err != nil {
			log.Printf("failed to fetch page at start=%d: %v", i, err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			log.Printf("failed to read page body at start=%d: %v", i, err)
			continue
		}

		jobs, err := ParseJobs(string(body), models.LinkedIn)
		if err != nil {
			log.Printf("failed to parse page at start=%d: %v", i, err)
			continue
		}

		if len(jobs) == 0 {
			log.Printf("no jobs returned at start=%d, stopping early", i)
			break
		}

		found.Add(int64(len(jobs)))
		log.Printf("page start=%d: found %d jobs", i, len(jobs))
		for _, job := range jobs {
			jobsCh <- job
		}

		randomDelay()
	}
}

// processJob upserts the listing and, for new jobs, fetches and saves the detail page.
// Returns (true, nil) if saved, (false, nil) if already seen, or (false, err) on failure.
func processJob(ctx context.Context, pool *pgxpool.Pool, job models.Job, workType models.WorkType) (bool, error) {
	jobID, isNew, err := db.UpsertJob(ctx, pool, job)
	if err != nil {
		return false, fmt.Errorf("upsert: %w", err)
	}
	if !isNew {
		return false, nil
	}

	baseLog := models.RequestLog{
		Source:         models.LinkedIn,
		JobSourceID:    job.SourceID,
		URL:            jobPostingBaseURL + job.SourceID,
		RequestHeaders: "{}",
	}

	resp, err := SearchJobId(job.SourceID)
	if err != nil {
		entry := baseLog
		entry.Error = err.Error()
		entry.Message = "network error fetching job detail"
		entry.IsIssue = true
		db.InsertRequestLog(ctx, pool, entry)
		return false, fmt.Errorf("fetch detail: %w", err)
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
		return false, fmt.Errorf("read body: %w", err)
	}
	bodyStr := string(body)

	if statusCode != http.StatusOK {
		entry := baseLog
		entry.Message = fmt.Sprintf("non-200 status code: %d", statusCode)
		entry.ResponseBody = bodyStr
		entry.IsIssue = true
		db.InsertRequestLog(ctx, pool, entry)
		return false, fmt.Errorf("unexpected status %d for job %s", statusCode, job.SourceID)
	}

	detail, err := ParseJobDetail(bodyStr)
	if err != nil {
		entry := baseLog
		entry.Error = err.Error()
		entry.Message = "failed to parse job detail HTML"
		entry.ResponseBody = bodyStr
		entry.IsIssue = true
		db.InsertRequestLog(ctx, pool, entry)
		return false, fmt.Errorf("parse detail: %w", err)
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

	if err := db.InsertJobDetail(ctx, pool, jobID, job, detail); err != nil {
		return false, fmt.Errorf("insert detail: %w", err)
	}

	return true, nil
}
