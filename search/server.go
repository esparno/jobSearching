package main

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"search/proto"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var excludedCompanies = []string{"Jobright.ai", "Jobot", "Joblet-AI", "Ladders"}

type server struct {
	proto.UnimplementedJobSearchServer
	pool *pgxpool.Pool
}

const query = `
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
           array_agg(DISTINCT jd.apply_url) AS apply_urls,
           min(j.posted_date)           AS posted_date,
           min(jd.description)           AS description
    FROM jobs j
    JOIN job_details jd ON j.id = jd.job_id
    WHERE ($1 = '' OR j.title !~* $1)
      AND j.company != ALL($2::text[])
      AND jd.description_tsv @@ to_tsquery('simple', $3)
    GROUP BY j.title, j.company, jd.pay_text
    ORDER BY dup_count DESC
)
SELECT * FROM deduped
ORDER BY posted_date DESC, pay_max DESC NULLS LAST`

func (s *server) Search(ctx context.Context, req *proto.SearchRequest) (*proto.SearchResponse, error) {
	if len(req.SearchTerms) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one search term is required")
	}

	tsquery, err := buildTsquery(req.SearchTerms)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	titlePattern := buildTitlePattern(req.TitleExclusions)

	rows, err := s.pool.Query(ctx, query, titlePattern, excludedCompanies, tsquery)
	if err != nil {
		log.Printf("search query: %v", err)
		return nil, status.Error(codes.Internal, "search failed")
	}
	defer rows.Close()

	var jobs []*proto.JobResult
	for rows.Next() {
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
		if err := rows.Scan(
			&title, &company, &payMin, &payMax, &payType,
			&dupCount, &maxApplicants, &minApplicants,
			&firstSeen, &lastSeen, &applyURLs, &postedDate,
			&description,
		); err != nil {
			log.Printf("search scan: %v", err)
			return nil, status.Error(codes.Internal, "search failed")
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

		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		log.Printf("search rows: %v", err)
		return nil, status.Error(codes.Internal, "search failed")
	}

	return &proto.SearchResponse{Jobs: jobs}, nil
}

func buildTsquery(terms []string) (string, error) {
	nonWord := regexp.MustCompile(`\W+`)
	parts := make([]string, 0, len(terms))
	for _, t := range terms {
		t = strings.TrimSpace(t)
		t = nonWord.ReplaceAllString(t, "")
		if t != "" {
			parts = append(parts, t)
		}
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("no valid search terms after sanitization")
	}
	return strings.Join(parts, " & "), nil
}

func buildTitlePattern(exclusions []string) string {
	if len(exclusions) == 0 {
		return ""
	}
	escaped := make([]string, 0, len(exclusions))
	for _, e := range exclusions {
		e = strings.TrimSpace(e)
		if e != "" {
			escaped = append(escaped, regexp.QuoteMeta(e))
		}
	}
	return strings.Join(escaped, "|")
}

// filterNilStrings removes nil placeholder values postgres may include in array_agg.
func filterNilStrings(ss []string) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
