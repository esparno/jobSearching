package main

import (
	"context"

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

func (s *server) Search(ctx context.Context, req *proto.SearchRequest) (*proto.SearchResponse, error) {
	if len(req.ExactSearchTerms) == 0 && len(req.NonExactSearchTerms) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one of exactTerms or nonExactTerms is required")
	}

	exactTsquery, err := buildTsquery(req.ExactSearchTerms)
	if err != nil && len(req.ExactSearchTerms) > 0 {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	nonExactTsquery, err := buildTsquery(req.NonExactSearchTerms)
	if err != nil && len(req.NonExactSearchTerms) > 0 {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	companies := append(excludedCompanies, req.CompanyExclusions...)

	jobs, err := s.queryJobs(ctx,
		buildTitlePattern(req.TitleExclusions),
		exactTsquery,
		nonExactTsquery,
		buildDescriptionExclusions(req.DescriptionExclusions),
		!req.ExcludeNullPay,
		req.PayMin, req.MaxApplicants, req.Days,
		companies,
	)
	if err != nil {
		return nil, err
	}

	return &proto.SearchResponse{Jobs: jobs}, nil
}

func (s *server) queryJobs(ctx context.Context, titlePattern, exactTsquery, nonExactTsquery, descExclusions string, includeNullPay bool, payMin, maxApplicants, days uint32, companies []string) ([]*proto.JobResult, error) {
	rows, err := s.pool.Query(ctx, searchQuery,
		titlePattern, companies, exactTsquery, includeNullPay,
		payMin, maxApplicants, days, descExclusions, nonExactTsquery,
	)
	if err != nil {
		return nil, internalErr("search query: %v", err)
	}
	defer rows.Close()

	var jobs []*proto.JobResult
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, internalErr("search scan: %v", err)
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, internalErr("search rows: %v", err)
	}

	return jobs, nil
}
