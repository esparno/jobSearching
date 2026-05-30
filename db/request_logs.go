package db

import (
	"context"
	"jobSearching/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

// GetFailedJobsByRunID returns jobs from the given run that have at least one is_issue=true
// request log entry and still lack a job_details record.
func GetFailedJobsByRunID(ctx context.Context, pool *pgxpool.Pool, runID string) ([]models.Job, error) {
	rows, err := pool.Query(ctx, `
		SELECT DISTINCT j.id, j.source, j.source_id, j.title, j.company, j.location, j.url,
		       COALESCE(j.posted_date::text, ''), j.run_id
		FROM jobs j
		JOIN request_logs rl ON rl.job_source_id = j.source_id AND rl.source = j.source
		WHERE rl.run_id = $1
		AND rl.is_issue = true
		AND NOT EXISTS (SELECT 1 FROM job_details jd WHERE jd.job_id = j.id)
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		var (
			job                                        models.Job
			title, company, location, url, postedDate *string
		)
		if err := rows.Scan(
			&job.ID, &job.Source, &job.SourceID,
			&title, &company, &location, &url, &postedDate, &job.RunID,
		); err != nil {
			return nil, err
		}
		if title != nil {
			job.Title = *title
		}
		if company != nil {
			job.Company = *company
		}
		if location != nil {
			job.Location = *location
		}
		if url != nil {
			job.URL = *url
		}
		if postedDate != nil {
			job.PostedDate = *postedDate
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

// GetFailedSearchURLsByRunID returns the URLs of search-page requests that failed during the given run.
func GetFailedSearchURLsByRunID(ctx context.Context, pool *pgxpool.Pool, runID string) ([]string, error) {
	rows, err := pool.Query(ctx, `
		SELECT url FROM request_logs
		WHERE run_id = $1
		AND is_issue = true
		AND job_source_id IS NULL
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var urls []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		urls = append(urls, u)
	}
	return urls, rows.Err()
}

// InsertRequestLog records an outbound HTTP request and its outcome.
// Errors are written to stderr rather than returned so a logging failure never interrupts a scrape.
// A nil pool is silently ignored.
func InsertRequestLog(ctx context.Context, pool *pgxpool.Pool, entry models.RequestLog) {
	if pool == nil {
		return
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO request_logs (
			source, job_source_id, url, request_headers,
			status_code, error, message, response_body, is_issue, run_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`,
		entry.Source,
		nullableString(entry.JobSourceID),
		entry.URL,
		entry.RequestHeaders,
		entry.StatusCode,
		nullableString(entry.Error),
		entry.Message,
		nullableString(entry.ResponseBody),
		entry.IsIssue,
		nullableString(entry.RunID),
	)
	if err != nil {
		// Log to stderr so a logging failure never interrupts the scrape
		println("failed to insert request log:", err.Error())
	}
}