package main

import (
	"context"
	"fmt"
	"io"
	"jobSearching/api"
	"jobSearching/db"
	"log"

	"github.com/joho/godotenv"
)

func main() {
	logFatal(godotenv.Load())

	ctx := context.Background()

	pool, err := db.Connect(ctx)
	logFatal(err)
	defer pool.Close()

	// Search for jobs
	searchOptions := api.SearchOptions{}
	resp, err := api.SearchJobs(api.KeywordsSoftwareEngineer, api.OneDay, searchOptions)
	logFatal(err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	logFatal(err)

	jobs, err := api.ParseJobs(string(body), "linkedin")
	logFatal(err)

	fmt.Printf("Found %d jobs\n", len(jobs))

	for _, job := range jobs {
		jobID, err := db.UpsertJob(ctx, pool, job)
		if err != nil {
			log.Printf("failed to upsert job %s: %v", job.SourceID, err)
			continue
		}

		// Fetch and save details for each job
		detailResp, err := api.SearchJobId(job.SourceID)
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

		detail, err := api.ParseJobDetail(string(detailBody))
		if err != nil {
			log.Printf("failed to parse detail for job %s: %v", job.SourceID, err)
			continue
		}

		if err := db.InsertJobDetail(ctx, pool, jobID, job, detail); err != nil {
			log.Printf("failed to insert detail for job %s: %v", job.SourceID, err)
			continue
		}

		fmt.Printf("saved: %s | %s | %s\n", job.SourceID, job.Title, job.Company)
	}
}

func logFatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
