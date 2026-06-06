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
	if len(req.SearchTerms) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one search term is required")
	}

	tsquery, err := buildTsquery(req.SearchTerms)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	companies := append(excludedCompanies, req.CompanyExclusions...)

	jobs, err := s.queryJobs(ctx, buildTitlePattern(req.TitleExclusions), tsquery, buildDescriptionExclusions(req.DescriptionExclusions), !req.ExcludeNullPay, req.PayMin, req.MaxApplicants, req.Days, companies)
	if err != nil {
		return nil, err
	}

	return &proto.SearchResponse{Jobs: jobs}, nil
}

func (s *server) queryJobs(ctx context.Context, titlePattern, tsquery, descExclusions string, includeNullPay bool, payMin, maxApplicants, days uint32, companies []string) ([]*proto.JobResult, error) {
	rows, err := s.pool.Query(ctx, searchQuery, titlePattern, companies, tsquery, includeNullPay, payMin, maxApplicants, days, descExclusions)
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
