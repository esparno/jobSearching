package main

import (
	"flag"
	"fmt"
	"jobSearching/api/linkedin"
	"jobSearching/models"
	"os"
	"strconv"
)

type config struct {
	opts    linkedin.SearchOptions
	numJobs int
}

func parseConfig(args []string) (config, error) {
	fs := flag.NewFlagSet("jobSearching", flag.ContinueOnError)
	keywords := fs.String("keywords", "", "search keywords (e.g. software+engineer)")
	timePosted := fs.String("time-posted", "", "time posted filter: r<seconds> (e.g. r86400 for 24h)")
	workType := fs.String("work-type", "", "work type: remote, onsite, hybrid")
	jobType := fs.String("job-type", "", "job type: F, P, C, T (optional)")
	numJobs := fs.Int("jobs", 0, "number of jobs to scrape (default 10)")
	start := fs.Int("start", 0, "pagination start offset (default 0)")

	if err := fs.Parse(args); err != nil {
		return config{}, err
	}

	flagOrEnv := func(val *string, env string) string {
		if *val != "" {
			return *val
		}
		return os.Getenv(env)
	}

	numJobsVal := *numJobs
	if numJobsVal == 0 {
		if v := os.Getenv("NUM_JOBS"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				return config{}, fmt.Errorf("invalid NUM_JOBS: %v", err)
			}
			numJobsVal = n
		}
	}
	if numJobsVal == 0 {
		numJobsVal = 10
	}

	startVal := *start
	if startVal == 0 {
		if v := os.Getenv("START"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				return config{}, fmt.Errorf("invalid START: %v", err)
			}
			startVal = n
		}
	}

	return config{
		numJobs: numJobsVal,
		opts: linkedin.SearchOptions{
			Keywords:   linkedin.Keywords(flagOrEnv(keywords, "KEYWORDS")),
			TimePosted: linkedin.TimeFilter(flagOrEnv(timePosted, "TIME_POSTED")),
			WorkType:   models.WorkType(flagOrEnv(workType, "WORK_TYPE")),
			JobType:    linkedin.JobType(flagOrEnv(jobType, "JOB_TYPE")),
			Start:      startVal,
		},
	}, nil
}
