package main

import (
	"time"

	"search/proto"

	"github.com/jackc/pgx/v5"
)

const searchQuery = `
WITH deduped AS (
    SELECT j.title,
           j.company,
           min(jd.pay_min)              AS pay_min,
           max(jd.pay_max)              AS pay_max,
           min(jd.pay_type)             AS pay_type,
           count(*)                     AS dup_count,
           max(jd.applicants)           AS max_applicants,
           min(jd.applicants)           AS min_applicants,
           min(j.first_seen)            AS first_seen,
           max(j.last_seen)             AS last_seen,
           array_agg(DISTINCT jd.apply_url) FILTER (WHERE jd.apply_url IS NOT NULL) AS apply_urls,
           min(j.posted_date)           AS posted_date,
           min(jd.description)          AS description
    FROM jobs j
    JOIN job_details jd ON j.id = jd.job_id
    WHERE ($1 = '' OR j.title !~* $1)
      AND j.company != ALL($2::text[])
      AND ($3 = '' OR jd.description_tsv @@ to_tsquery('simple', $3))
      AND ($9 = '' OR jd.description_tsv @@ to_tsquery('english', $9))
      AND ($4 OR jd.pay_text IS NOT NULL)
	  AND ($5 = 0 OR jd.pay_max >= $5 OR jd.pay_min >= $5 OR jd.pay_text IS NULL)
	  AND ($6 = 0 OR jd.applicants IS NULL OR jd.applicants <= $6)
	  AND ($7 = 0 OR j.posted_date >= CURRENT_DATE - ($7::int - 1))
	  AND ($8 = '' OR NOT (jd.description_tsv @@ to_tsquery('english', $8)))
    GROUP BY j.title, j.company, jd.pay_text
    ORDER BY dup_count DESC
)
SELECT * FROM deduped
ORDER BY posted_date DESC, pay_max DESC NULLS LAST`

func scanJob(row pgx.Row) (*proto.JobResult, error) {
	var (
		title         string
		company       string
		payMin        *float64
		payMax        *float64
		payType       *string
		dupCount      int32
		maxApplicants *int32
		minApplicants *int32
		firstSeen     time.Time
		lastSeen      time.Time
		applyURLs     []string
		postedDate    *time.Time
		description   string
	)
	if err := row.Scan(
		&title, &company, &payMin, &payMax, &payType,
		&dupCount, &maxApplicants, &minApplicants,
		&firstSeen, &lastSeen, &applyURLs, &postedDate,
		&description,
	); err != nil {
		return nil, err
	}

	job := &proto.JobResult{
		Title:       title,
		Company:     company,
		DupCount:    dupCount,
		FirstSeen:   firstSeen.Format(time.RFC3339),
		LastSeen:    lastSeen.Format(time.RFC3339),
		ApplyUrls:   filterNilStrings(applyURLs),
		Description: description,
	}
	if payMin != nil {
		job.PayMin = payMin
	}
	if payMax != nil {
		job.PayMax = payMax
	}
	if payType != nil {
		job.PayType = *payType
	}
	if maxApplicants != nil {
		job.MaxApplicants = maxApplicants
	}
	if minApplicants != nil {
		job.MinApplicants = minApplicants
	}
	if postedDate != nil {
		job.PostedDate = postedDate.Format("2006-01-02")
	}

	return job, nil
}
