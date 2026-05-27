package db

import (
	"context"
	"jobSearching/models"
	"testing"
	"time"
)

func TestInsertScrapeRun(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	start := time.Now().Truncate(time.Millisecond)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM scrape_runs WHERE started_at = $1", start)
	})

	run := models.ScrapeRun{
		Source:      models.LinkedIn,
		Keywords:    "software+engineer",
		TimePosted:  "r86400",
		WorkType:    "remote",
		JobType:     "F",
		StartedAt:   start,
		FinishedAt:  start.Add(5 * time.Minute),
		JobsFound:   42,
		JobsNew:     10,
		JobsSkipped: 32,
	}

	if err := InsertScrapeRun(ctx, pool, run); err != nil {
		t.Fatalf("InsertScrapeRun: %v", err)
	}

	var found, newJobs, skipped int64
	err := pool.QueryRow(ctx, `
		SELECT jobs_found, jobs_new, jobs_skipped FROM scrape_runs WHERE started_at = $1
	`, start).Scan(&found, &newJobs, &skipped)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if found != run.JobsFound || newJobs != run.JobsNew || skipped != run.JobsSkipped {
		t.Errorf("counts: got (%d/%d/%d), want (%d/%d/%d)",
			found, newJobs, skipped,
			run.JobsFound, run.JobsNew, run.JobsSkipped)
	}
}

func TestInsertScrapeRun_EmptyJobType(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	start := time.Now().Truncate(time.Millisecond)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM scrape_runs WHERE started_at = $1", start)
	})

	run := models.ScrapeRun{
		Source:     models.LinkedIn,
		Keywords:   "data+engineer",
		TimePosted: "r604800",
		WorkType:   "hybrid",
		StartedAt:  start,
		FinishedAt: start.Add(time.Minute),
	}

	if err := InsertScrapeRun(ctx, pool, run); err != nil {
		t.Fatalf("InsertScrapeRun: %v", err)
	}

	var jobType *string
	_ = pool.QueryRow(ctx, "SELECT job_type FROM scrape_runs WHERE started_at = $1", start).Scan(&jobType)
	if jobType != nil {
		t.Errorf("job_type: got %q, want nil (empty string should be NULL)", *jobType)
	}
}