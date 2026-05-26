package db

import (
	"context"
	"jobSearching/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

// UpsertJob inserts a job or updates last_seen if it already exists.
// Returns the internal job ID.
func UpsertJob(ctx context.Context, pool *pgxpool.Pool, job models.Job) (int64, error) {
	var id int64
	err := pool.QueryRow(ctx, `
		INSERT INTO jobs (source, source_id, title, company, location, url, posted_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7::date)
		ON CONFLICT (source, source_id) DO UPDATE SET last_seen = NOW()
		RETURNING id
	`,
		job.Source,
		job.SourceID,
		nullableString(job.Title),
		nullableString(job.Company),
		nullableString(job.Location),
		nullableString(job.URL),
		nullableString(job.PostedDate),
	).Scan(&id)

	return id, err
}

// InsertJobDetail saves the full detail for a job.
// Does nothing if details already exist for this job.
func InsertJobDetail(ctx context.Context, pool *pgxpool.Pool, jobID int64, job models.Job, detail models.JobDetail) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO job_details (
			job_id, source, source_id, seniority, employment_type, work_type,
			job_function, industries, description, applicants,
			salary_min, salary_max, salary_text
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (job_id) DO NOTHING
	`,
		jobID,
		job.Source,
		job.SourceID,
		nullableString(detail.Seniority),
		nullableString(detail.EmploymentType),
		nullableString(detail.WorkType),
		nullableString(detail.JobFunction),
		nullableString(detail.Industries),
		nullableString(detail.Description),
		nullableString(detail.Applicants),
		detail.SalaryMin,
		detail.SalaryMax,
		nullableString(detail.SalaryText),
	)
	return err
}

// nullableString returns nil for empty strings so they are stored as NULL.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
