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
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

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

func SearchJobs(searchOptions SearchOptions) (*http.Response, error) {
	if err := searchOptions.Validate(); err != nil {
		return nil, err
	}

	searchUrl := "https://www.linkedin.com/jobs-guest/jobs/api/seeMoreJobPostings/search"
	location := "United+States"
	geoId := "103644278" // geo id for United States

	params := url.Values{}
	params.Set("keywords", string(searchOptions.Keywords))
	params.Set("location", location)
	params.Set("geoId", geoId)
	params.Set("f_TPR", string(searchOptions.TimePosted))
	params.Set("f_WT", strconv.Itoa(workTypeValues[searchOptions.WorkType]))
	params.Set("start", strconv.Itoa(searchOptions.Start))

	if searchOptions.JobType != "" {
		params.Set("f_JT", string(searchOptions.JobType))
	}

	getUrl := searchUrl + "?" + params.Encode()
	fmt.Println(getUrl)
	return http.Get(getUrl)
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
	postingUrl := "https://www.linkedin.com/jobs-guest/jobs/api/jobPosting/" + jobId
	return http.Get(postingUrl)
}

const (
	numWorkers = 3
	minDelay   = 2 * time.Second
	maxJitter  = 3 * time.Second
)

func randomDelay() {
	time.Sleep(minDelay + time.Duration(rand.Int63n(int64(maxJitter))))
}

func ScrapeJobs(numberOfJobs int, opts SearchOptions, ctx context.Context, pool *pgxpool.Pool) {
	jobsCh := make(chan models.Job, (numberOfJobs-opts.Start)/10)

	// Paginator — fetches pages and sends jobs into the channel
	go func() {
		defer close(jobsCh)
		for i := opts.Start; i < numberOfJobs; i += 10 {
			searchOptions := opts
			searchOptions.Start = i
			resp, err := SearchJobs(searchOptions)
			if err != nil {
				log.Printf("failed to fetch page at start=%d: %v", searchOptions.Start, err)
				continue
			}

			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				log.Printf("failed to read page body at start=%d: %v", i, err)
				continue
			}

			jobs, err := ParseJobs(string(body), models.LinkedIn)
			if err != nil {
				log.Printf("failed to parse page at start=%d: %v", i, err)
				continue
			}

			fmt.Printf("page start=%d: found %d jobs\n", i, len(jobs))
			for _, job := range jobs {
				jobsCh <- job
			}

			randomDelay()
		}
	}()

	// Worker pool — each worker fetches details and saves to DB
	var wg sync.WaitGroup
	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobsCh {
				jobID, err := db.UpsertJob(ctx, pool, job)
				if err != nil {
					log.Printf("failed to upsert job %s: %v", job.SourceID, err)
					continue
				}

				detailResp, err := SearchJobId(job.SourceID)
				if err != nil {
					log.Printf("failed to fetch detail for job %s: %v", job.SourceID, err)
					continue
				}

				detailBody, err := io.ReadAll(detailResp.Body)
				detailResp.Body.Close()
				if err != nil {
					log.Printf("failed to read detail body for job %s: %v", job.SourceID, err)
					continue
				}

				detail, err := ParseJobDetail(string(detailBody))
				if err != nil {
					log.Printf("failed to parse detail for job %s: %v", job.SourceID, err)
					continue
				}

				if err := db.InsertJobDetail(ctx, pool, jobID, job, detail); err != nil {
					log.Printf("failed to insert detail for job %s: %v", job.SourceID, err)
					continue
				}

				fmt.Printf("saved: %s | %s | %s\n", job.SourceID, job.Title, job.Company)
				randomDelay()
			}
		}()
	}

	wg.Wait()
}
