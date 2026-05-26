package linkedin

import (
	"context"
	"fmt"
	"io"
	"jobSearching/db"
	"jobSearching/models"
	"jobSearching/source"
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

type workType int

const (
	Onsite workType = 1
	Remote workType = 2
	Hybrid workType = 3
)

type jobType string

const (
	FullTime  jobType = "F"
	PartTime  jobType = "P"
	Contract  jobType = "C"
	Temporary jobType = "T"
)

type SearchOptions struct {
	JobType  jobType
	WorkType workType
	Start    int
}

//location=New+York
//locationId=         # LinkedIn's internal geo ID
//geoId=              # alternative geo identifier
//f_TPR=r86400        # time posted: r3600=1hr, r86400=24hr, r604800=week
//f_WT=2              # work type: 1=onsite, 2=remote, 3=hybrid
//f_E=2,3             # experience: 1=internship, 2=entry, 3=associate, 4=mid, 5=director, 6=executive
//f_JT=F,P,C          # job type: F=fulltime, P=parttime, C=contract, T=temporary
//start=0             # pagination offset, increments of 10

func SearchJobs(searchTerm Keywords, timePeriod timePostedLookup, searchOptions SearchOptions) (*http.Response, error) {
	searchUrl := "https://www.linkedin.com/jobs-guest/jobs/api/seeMoreJobPostings/search"
	location := "United+States"
	geoId := "103644278" // geo id for United States

	params := url.Values{}
	params.Set("keywords", string(searchTerm))
	params.Set("location", location)
	params.Set("geoId", geoId)
	params.Set("f_TPR", string(timePeriod))
	searchOptions.AddSearchParams(params)

	getUrl := searchUrl + "?" + params.Encode()
	fmt.Println(getUrl)
	return http.Get(getUrl)
}

func (searchOptions SearchOptions) AddSearchParams(params url.Values) {
	if searchOptions.JobType != "" {
		params.Set("f_JT", string(searchOptions.JobType))
	}
	if searchOptions.WorkType > 0 {
		params.Set("f_WT", strconv.Itoa(int(searchOptions.WorkType)))
	}
	params.Set("start", strconv.Itoa(searchOptions.Start))
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

func ScrapeJobs(numberOfJobs int, start int, ctx context.Context, pool *pgxpool.Pool) {
	jobsCh := make(chan models.Job, (numberOfJobs-start)/10)

	// Paginator — fetches pages and sends jobs into the channel
	go func() {
		defer close(jobsCh)
		for i := start; i < numberOfJobs; i += 10 {
			searchOptions := SearchOptions{Start: i}
			resp, err := SearchJobs(KeywordsSoftwareEngineer, OneDay, searchOptions)
			if err != nil {
				log.Printf("failed to fetch page at start=%d: %v", i, err)
				continue
			}

			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				log.Printf("failed to read page body at start=%d: %v", i, err)
				continue
			}

			jobs, err := ParseJobs(string(body), source.LinkedIn)
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

func logFatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
