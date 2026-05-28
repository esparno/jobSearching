package db

import (
	"context"
	"jobSearching/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

// InsertScrapeRun records a completed scrape run with its aggregate counts and search parameters.
func InsertScrapeRun(ctx context.Context, pool *pgxpool.Pool, run models.ScrapeRun) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO scrape_runs (
			source, keywords, time_posted, work_type, job_type,
			started_at, finished_at, jobs_found, jobs_new, jobs_skipped
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`,
		run.Source,
		run.Keywords,
		run.TimePosted,
		run.WorkType,
		nullableString(run.JobType),
		run.StartedAt,
		run.FinishedAt,
		run.JobsFound,
		run.JobsNew,
		run.JobsSkipped,
	)
	return err
}
