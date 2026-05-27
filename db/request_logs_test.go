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