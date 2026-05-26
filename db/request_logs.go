package db

import (
	"context"
	"jobSearching/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

func InsertRequestLog(ctx context.Context, pool *pgxpool.Pool, entry models.RequestLog) {
	_, err := pool.Exec(ctx, `
		INSERT INTO request_logs (
			source, job_source_id, url, request_headers,
			status_code, error, message, response_body, is_issue
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
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
	)
	if err != nil {
		// Log to stderr so a logging failure never interrupts the scrape
		println("failed to insert request log:", err.Error())
	}
}