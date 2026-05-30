package db

import (
	"context"
	"jobSearching/models"
	"testing"
)

func TestInsertRequestLog(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	sourceID := testSourceID(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM request_logs WHERE job_source_id = $1", sourceID)
	})

	statusCode := 200
	InsertRequestLog(ctx, pool, models.RequestLog{
		Source:         models.LinkedIn,
		JobSourceID:    sourceID,
		URL:            "https://example.com/jobs/" + sourceID,
		RequestHeaders: "{}",
		StatusCode:     &statusCode,
		Message:        "ok",
		IsIssue:        false,
	})

	var count int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM request_logs WHERE job_source_id = $1", sourceID).Scan(&count)
	if count != 1 {
		t.Errorf("row count: got %d, want 1", count)
	}
}

func TestInsertRequestLog_WithError(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	sourceID := testSourceID(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM request_logs WHERE job_source_id = $1", sourceID)
	})

	InsertRequestLog(ctx, pool, models.RequestLog{
		Source:         models.LinkedIn,
		JobSourceID:    sourceID,
		URL:            "https://example.com/jobs/" + sourceID,
		RequestHeaders: "{}",
		Error:          "connection refused",
		Message:        "network error fetching job detail",
		IsIssue:        true,
	})

	var isIssue bool
	var storedErr string
	_ = pool.QueryRow(ctx, `
		SELECT is_issue, error FROM request_logs WHERE job_source_id = $1
	`, sourceID).Scan(&isIssue, &storedErr)
	if !isIssue {
		t.Error("is_issue: got false, want true")
	}
	if storedErr != "connection refused" {
		t.Errorf("error: got %q, want %q", storedErr, "connection refused")
	}
}

func TestInsertRequestLog_StoresRunID(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	sourceID := testSourceID(t)
	runID := testRunID(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM request_logs WHERE job_source_id = $1", sourceID)
	})

	InsertRequestLog(ctx, pool, models.RequestLog{
		Source:         models.LinkedIn,
		JobSourceID:    sourceID,
		URL:            "https://example.com/jobs/" + sourceID,
		RequestHeaders: "{}",
		Message:        "ok",
		RunID:          runID,
	})

	var storedRunID string
	_ = pool.QueryRow(ctx, "SELECT run_id FROM request_logs WHERE job_source_id = $1", sourceID).Scan(&storedRunID)
	if storedRunID != runID {
		t.Errorf("run_id: got %q, want %q", storedRunID, runID)
	}
}

func TestGetFailedJobsByRunID(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	sourceID := testSourceID(t)
	runID := testRunID(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM request_logs WHERE job_source_id = $1", sourceID)
		_, _ = pool.Exec(ctx, "DELETE FROM jobs WHERE source_id = $1", sourceID)
	})

	job := testJob(sourceID)
	job.RunID = runID
	id, _, err := UpsertJob(ctx, pool, job)
	if err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}
	job.ID = id

	InsertRequestLog(ctx, pool, models.RequestLog{
		Source:         models.LinkedIn,
		JobSourceID:    sourceID,
		URL:            "https://example.com/jobs/" + sourceID,
		RequestHeaders: "{}",
		Message:        "non-200 status code: 429",
		IsIssue:        true,
		RunID:          runID,
	})

	jobs, err := GetFailedJobsByRunID(ctx, pool, runID)
	if err != nil {
		t.Fatalf("GetFailedJobsByRunID: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("len: got %d, want 1", len(jobs))
	}
	if jobs[0].SourceID != sourceID {
		t.Errorf("source_id: got %q, want %q", jobs[0].SourceID, sourceID)
	}
}

func TestGetFailedJobsByRunID_ExcludesJobsWithDetails(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	sourceID := testSourceID(t)
	runID := testRunID(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM request_logs WHERE job_source_id = $1", sourceID)
		_, _ = pool.Exec(ctx, "DELETE FROM job_details WHERE source_id = $1", sourceID)
		_, _ = pool.Exec(ctx, "DELETE FROM jobs WHERE source_id = $1", sourceID)
	})

	job := testJob(sourceID)
	job.RunID = runID
	id, _, err := UpsertJob(ctx, pool, job)
	if err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}
	job.ID = id

	InsertRequestLog(ctx, pool, models.RequestLog{
		Source:         models.LinkedIn,
		JobSourceID:    sourceID,
		URL:            "https://example.com/jobs/" + sourceID,
		RequestHeaders: "{}",
		Message:        "non-200 status code: 429",
		IsIssue:        true,
		RunID:          runID,
	})

	if err := InsertJobDetail(ctx, pool, job, models.JobDetail{SourceID: sourceID, Description: "details"}); err != nil {
		t.Fatalf("InsertJobDetail: %v", err)
	}

	jobs, err := GetFailedJobsByRunID(ctx, pool, runID)
	if err != nil {
		t.Fatalf("GetFailedJobsByRunID: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("len: got %d, want 0 (job with detail should be excluded)", len(jobs))
	}
}

func TestGetFailedJobsByRunID_ExcludesOtherRuns(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	sourceID := testSourceID(t)
	runID := testRunID(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM request_logs WHERE job_source_id = $1", sourceID)
		_, _ = pool.Exec(ctx, "DELETE FROM jobs WHERE source_id = $1", sourceID)
	})

	job := testJob(sourceID)
	job.RunID = runID
	if _, _, err := UpsertJob(ctx, pool, job); err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}

	InsertRequestLog(ctx, pool, models.RequestLog{
		Source:         models.LinkedIn,
		JobSourceID:    sourceID,
		URL:            "https://example.com/jobs/" + sourceID,
		RequestHeaders: "{}",
		Message:        "non-200 status code: 429",
		IsIssue:        true,
		RunID:          runID,
	})

	jobs, err := GetFailedJobsByRunID(ctx, pool, "different-run-id")
	if err != nil {
		t.Fatalf("GetFailedJobsByRunID: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("len: got %d, want 0 (different run should be excluded)", len(jobs))
	}
}

func TestGetFailedSearchURLsByRunID(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	runID := testRunID(t)
	wantURL := "https://www.linkedin.com/jobs-guest/jobs/api/seeMoreJobPostings/search?start=20"
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM request_logs WHERE run_id = $1", runID)
	})

	InsertRequestLog(ctx, pool, models.RequestLog{
		Source:         models.LinkedIn,
		URL:            wantURL,
		RequestHeaders: "{}",
		Message:        "empty response from search page",
		IsIssue:        true,
		RunID:          runID,
	})

	urls, err := GetFailedSearchURLsByRunID(ctx, pool, runID)
	if err != nil {
		t.Fatalf("GetFailedSearchURLsByRunID: %v", err)
	}
	if len(urls) != 1 {
		t.Fatalf("len: got %d, want 1", len(urls))
	}
	if urls[0] != wantURL {
		t.Errorf("url: got %q, want %q", urls[0], wantURL)
	}
}

func TestGetFailedSearchURLsByRunID_ExcludesJobDetailLogs(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	sourceID := testSourceID(t)
	runID := testRunID(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM request_logs WHERE run_id = $1", runID)
	})

	// Log with a job_source_id — this is a detail fetch failure, not a search page failure.
	InsertRequestLog(ctx, pool, models.RequestLog{
		Source:         models.LinkedIn,
		JobSourceID:    sourceID,
		URL:            "https://example.com/jobs/" + sourceID,
		RequestHeaders: "{}",
		Message:        "non-200 status code: 429",
		IsIssue:        true,
		RunID:          runID,
	})

	urls, err := GetFailedSearchURLsByRunID(ctx, pool, runID)
	if err != nil {
		t.Fatalf("GetFailedSearchURLsByRunID: %v", err)
	}
	if len(urls) != 0 {
		t.Errorf("len: got %d, want 0 (detail logs should be excluded)", len(urls))
	}
}

func TestGetFailedSearchURLsByRunID_ExcludesSuccessful(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	runID := testRunID(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM request_logs WHERE run_id = $1", runID)
	})

	InsertRequestLog(ctx, pool, models.RequestLog{
		Source:         models.LinkedIn,
		URL:            "https://www.linkedin.com/jobs-guest/jobs/api/seeMoreJobPostings/search?start=0",
		RequestHeaders: "{}",
		Message:        "ok",
		IsIssue:        false,
		RunID:          runID,
	})

	urls, err := GetFailedSearchURLsByRunID(ctx, pool, runID)
	if err != nil {
		t.Fatalf("GetFailedSearchURLsByRunID: %v", err)
	}
	if len(urls) != 0 {
		t.Errorf("len: got %d, want 0 (successful logs should be excluded)", len(urls))
	}
}