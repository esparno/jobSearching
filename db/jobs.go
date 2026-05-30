package db

import (
	"context"
	"jobSearching/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

// UpsertJob inserts a job listing or bumps last_seen if it already exists.
// Returns the internal job ID, whether the row was newly inserted, and any error.
func UpsertJob(ctx context.Context, pool *pgxpool.Pool, job models.Job) (int64, bool, error) {
	var id int64
	var isNew bool
	err := pool.QueryRow(ctx, `
		INSERT INTO jobs (source, source_id, title, company, location, url, posted_date, run_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7::date, $8)
		ON CONFLICT (source, source_id) DO UPDATE SET last_seen = NOW(), run_id = EXCLUDED.run_id
		RETURNING id, (xmax = 0) AS is_new
	`,
		job.Source,
		job.SourceID,
		nullableString(job.Title),
		nullableString(job.Company),
		nullableString(job.Location),
		nullableString(job.URL),
		nullableString(job.PostedDate),
		job.RunID,
	).Scan(&id, &isNew)

	return id, isNew, err
}

// InsertJobDetail saves the full detail page data for a job.
// Does nothing if a detail row already exists for that job (ON CONFLICT DO NOTHING).
func InsertJobDetail(ctx context.Context, pool *pgxpool.Pool, job models.Job, detail models.JobDetail) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO job_details (
			job_id, source, source_id, seniority, employment_type, work_type,
			job_function, industries, description, applicants_text, applicants, applicants_qualifier,
			pay_type, pay_min, pay_max, pay_text, apply_url, run_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		ON CONFLICT (job_id) DO UPDATE SET
			applicants_text = EXCLUDED.applicants_text,
			applicants = EXCLUDED.applicants,
			applicants_qualifier = EXCLUDED.applicants_qualifier,
			run_id = EXCLUDED.run_id
		WHERE job_details.run_id != EXCLUDED.run_id
		AND (job_details.applicants IS NULL OR job_details.applicants < 200)
	`,
		job.ID,
		job.Source,
		job.SourceID,
		nullableString(detail.Seniority),
		nullableString(detail.EmploymentType),
		nullableString(detail.WorkType),
		nullableString(detail.JobFunction),
		nullableString(detail.Industries),
		nullableString(detail.Description),
		nullableString(detail.ApplicantsText),
		detail.Applicants,
		nullableString(string(detail.ApplicantsQualifier)),
		nullableString(string(detail.PayType)),
		detail.PayMin,
		detail.PayMax,
		nullableString(detail.PayText),
		nullableString(detail.ApplyURL),
		job.RunID,
	)
	return err
}

// GetNewJobsByRunID returns all jobs inserted during the given run that do not yet have a detail record.
func GetNewJobsByRunID(ctx context.Context, pool *pgxpool.Pool, runID string) ([]models.Job, error) {
	rows, err := pool.Query(ctx, `
		SELECT j.id, j.source, j.source_id, j.run_id
		FROM jobs j
		LEFT JOIN job_details jd ON jd.job_id = j.id
		WHERE j.run_id = $1
		AND jd.id IS NULL
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		var job models.Job
		if err := rows.Scan(&job.ID, &job.Source, &job.SourceID, &job.RunID); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

// GetJobsForApplicantUpdateByRunID returns jobs from the current run that already have a
// detail record but whose applicant count is still below the 200-applicant cap.
func GetJobsForApplicantUpdateByRunID(ctx context.Context, pool *pgxpool.Pool, runID string) ([]models.Job, error) {
	rows, err := pool.Query(ctx, `
		SELECT j.id, j.source, j.source_id, j.run_id
		FROM jobs j
		JOIN job_details jd ON jd.job_id = j.id
		WHERE j.run_id = $1
		AND (jd.applicants IS NULL OR jd.applicants < 200)
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		var job models.Job
		if err := rows.Scan(&job.ID, &job.Source, &job.SourceID, &job.RunID); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

// nullableString returns nil for empty strings so that optional fields are stored as SQL NULL.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
