package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"search/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func containsCompany(jobs []*proto.JobResult, company string) bool {
	for _, j := range jobs {
		if j.Company == company {
			return true
		}
	}
	return false
}

func TestSearch_RequiresTerms(t *testing.T) {
	s := testServer(t)
	_, err := s.Search(context.Background(), &proto.SearchRequest{})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestSearch_InvalidExactSearchTermsAfterSanitization(t *testing.T) {
	s := testServer(t)
	_, err := s.Search(context.Background(), &proto.SearchRequest{
		ExactSearchTerms: []string{"!!!", "---"},
	})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestSearch_InvalidNonExactSearchTermsAfterSanitization(t *testing.T) {
	s := testServer(t)
	_, err := s.Search(context.Background(), &proto.SearchRequest{
		NonExactSearchTerms: []string{"!!!", "---"},
	})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestSearch_TermTooLong(t *testing.T) {
	s := testServer(t)
	long := strings.Repeat("a", maxTermLen+1)
	_, err := s.Search(context.Background(), &proto.SearchRequest{
		ExactSearchTerms: []string{long},
	})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for oversized term, got %v", err)
	}
}

func TestSearch_TooManyTerms(t *testing.T) {
	s := testServer(t)
	terms := make([]string, maxTermCount+1)
	for i := range terms {
		terms[i] = "scala"
	}
	_, err := s.Search(context.Background(), &proto.SearchRequest{
		ExactSearchTerms: terms,
	})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for too many terms, got %v", err)
	}
}

func TestSearch_TooManyTitleExclusions(t *testing.T) {
	s := testServer(t)
	excl := make([]string, maxTitleExclCount+1)
	for i := range excl {
		excl[i] = "Senior"
	}
	_, err := s.Search(context.Background(), &proto.SearchRequest{
		ExactSearchTerms: []string{"scala"},
		TitleExclusions:  excl,
	})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for too many title exclusions, got %v", err)
	}
}

func TestSearch_ExactSearchTerms(t *testing.T) {
	s := testServer(t)
	insertJob(t, s.pool, jobSpec{
		title:       "Scala Developer",
		company:     "ExactMatchCo",
		description: "We need a Scala developer.",
		postedDate:  time.Now(),
	})

	resp, err := s.Search(context.Background(), &proto.SearchRequest{
		ExactSearchTerms: []string{"scala"},
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !containsCompany(resp.Jobs, "ExactMatchCo") {
		t.Error("expected ExactMatchCo in results")
	}
}

func TestSearch_ExactSearchTerms_DoesNotStem(t *testing.T) {
	s := testServer(t)
	insertJob(t, s.pool, jobSpec{
		title:       "Scala Developer",
		company:     "ExactStemCo",
		description: "Looking for a Scala developer to develop features.",
		postedDate:  time.Now(),
	})

	// "developing" is stored as "develop" in the english tsvector.
	// exact search for "developing" should not match since simple tsquery doesn't stem.
	resp, err := s.Search(context.Background(), &proto.SearchRequest{
		ExactSearchTerms: []string{"developing"},
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if containsCompany(resp.Jobs, "ExactStemCo") {
		t.Error("ExactStemCo should not match: exact search should not stem 'developing' to match 'develop'")
	}
}

func TestSearch_NonExactSearchTerms(t *testing.T) {
	s := testServer(t)
	insertJob(t, s.pool, jobSpec{
		title:       "Scala Developer",
		company:     "NonExactMatchCo",
		description: "We need a Scala developer.",
		postedDate:  time.Now(),
	})

	resp, err := s.Search(context.Background(), &proto.SearchRequest{
		NonExactSearchTerms: []string{"scala"},
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !containsCompany(resp.Jobs, "NonExactMatchCo") {
		t.Error("expected NonExactMatchCo in results")
	}
}

func TestSearch_NonExactSearchTerms_Stems(t *testing.T) {
	s := testServer(t)
	insertJob(t, s.pool, jobSpec{
		title:       "Scala Developer",
		company:     "NonExactStemCo",
		description: "Looking for a Scala developer to develop features.",
		postedDate:  time.Now(),
	})

	// "developing" stems to "develop" with english, which matches "developer" and "develop" in the description.
	resp, err := s.Search(context.Background(), &proto.SearchRequest{
		NonExactSearchTerms: []string{"developing"},
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !containsCompany(resp.Jobs, "NonExactStemCo") {
		t.Error("expected NonExactStemCo: non-exact search should stem 'developing' to match 'develop'")
	}
}

func TestSearch_BothTerms_MustSatisfyBoth(t *testing.T) {
	s := testServer(t)
	insertJob(t, s.pool, jobSpec{
		title:       "Scala Developer",
		company:     "BothMatchCo",
		description: "Scala developer building distributed systems.",
		postedDate:  time.Now(),
	})
	insertJob(t, s.pool, jobSpec{
		title:       "Scala Manager",
		company:     "ExactOnlyCo",
		description: "Scala team lead and manager.",
		postedDate:  time.Now(),
	})
	insertJob(t, s.pool, jobSpec{
		title:       "Python Developer",
		company:     "NonExactOnlyCo",
		description: "Python developer building distributed systems.",
		postedDate:  time.Now(),
	})

	// only BothMatchCo satisfies both: exact 'scala' AND non-exact 'develop' (stems to match 'developer')
	resp, err := s.Search(context.Background(), &proto.SearchRequest{
		ExactSearchTerms:    []string{"scala"},
		NonExactSearchTerms: []string{"develop"},
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !containsCompany(resp.Jobs, "BothMatchCo") {
		t.Error("expected BothMatchCo in results")
	}
	if containsCompany(resp.Jobs, "ExactOnlyCo") {
		t.Error("ExactOnlyCo should be excluded: does not match NonExactSearchTerms")
	}
	if containsCompany(resp.Jobs, "NonExactOnlyCo") {
		t.Error("NonExactOnlyCo should be excluded: does not match ExactSearchTerms")
	}
}

func TestSearch_TitleExclusion(t *testing.T) {
	s := testServer(t)
	insertJob(t, s.pool, jobSpec{
		title:       "Senior Scala Developer",
		company:     "SeniorExcludedCo",
		description: "Senior Scala role.",
		postedDate:  time.Now(),
	})
	insertJob(t, s.pool, jobSpec{
		title:       "Scala Developer",
		company:     "JuniorIncludedCo",
		description: "Scala developer role.",
		postedDate:  time.Now(),
	})

	resp, err := s.Search(context.Background(), &proto.SearchRequest{
		ExactSearchTerms: []string{"scala"},
		TitleExclusions:  []string{"Senior"},
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if containsCompany(resp.Jobs, "SeniorExcludedCo") {
		t.Error("SeniorExcludedCo should be excluded by title filter")
	}
	if !containsCompany(resp.Jobs, "JuniorIncludedCo") {
		t.Error("expected JuniorIncludedCo in results")
	}
}

func TestSearch_CompanyExclusion(t *testing.T) {
	s := testServer(t)
	insertJob(t, s.pool, jobSpec{
		title:       "Scala Developer",
		company:     "UserExcludedCo",
		description: "Scala developer.",
		postedDate:  time.Now(),
	})
	insertJob(t, s.pool, jobSpec{
		title:       "Scala Developer",
		company:     "UserIncludedCo",
		description: "Scala developer.",
		postedDate:  time.Now(),
	})

	resp, err := s.Search(context.Background(), &proto.SearchRequest{
		ExactSearchTerms:  []string{"scala"},
		CompanyExclusions: []string{"UserExcludedCo"},
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if containsCompany(resp.Jobs, "UserExcludedCo") {
		t.Error("UserExcludedCo should be excluded")
	}
	if !containsCompany(resp.Jobs, "UserIncludedCo") {
		t.Error("expected UserIncludedCo in results")
	}
}

func TestSearch_HardcodedCompaniesExcluded(t *testing.T) {
	s := testServer(t)
	insertJob(t, s.pool, jobSpec{
		title:       "Scala Developer",
		company:     "Jobright.ai",
		description: "Scala developer.",
		postedDate:  time.Now(),
	})
	insertJob(t, s.pool, jobSpec{
		title:       "Scala Developer",
		company:     "HardcodedIncludedCo",
		description: "Scala developer.",
		postedDate:  time.Now(),
	})

	resp, err := s.Search(context.Background(), &proto.SearchRequest{
		ExactSearchTerms: []string{"scala"},
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if containsCompany(resp.Jobs, "Jobright.ai") {
		t.Error("Jobright.ai should always be excluded")
	}
	if !containsCompany(resp.Jobs, "HardcodedIncludedCo") {
		t.Error("expected HardcodedIncludedCo in results")
	}
}

func TestSearch_ExcludeNullPay(t *testing.T) {
	s := testServer(t)
	insertJob(t, s.pool, jobSpec{
		title:       "Scala Developer",
		company:     "WithPayCo",
		description: "Scala developer.",
		payText:     ptr("$100k"),
		postedDate:  time.Now(),
	})
	insertJob(t, s.pool, jobSpec{
		title:       "Scala Developer",
		company:     "NullPayCo",
		description: "Scala developer.",
		postedDate:  time.Now(),
	})

	resp, err := s.Search(context.Background(), &proto.SearchRequest{
		ExactSearchTerms: []string{"scala"},
		ExcludeNullPay:   true,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if containsCompany(resp.Jobs, "NullPayCo") {
		t.Error("NullPayCo should be excluded when ExcludeNullPay is true")
	}
	if !containsCompany(resp.Jobs, "WithPayCo") {
		t.Error("expected WithPayCo in results")
	}
}

func TestSearch_PayMin(t *testing.T) {
	s := testServer(t)
	insertJob(t, s.pool, jobSpec{
		title:       "Scala Developer",
		company:     "HighPayCo",
		description: "Scala developer.",
		payMax:      ptr(150000.0),
		payText:     ptr("$150k"),
		postedDate:  time.Now(),
	})
	insertJob(t, s.pool, jobSpec{
		title:       "Scala Developer",
		company:     "LowPayCo",
		description: "Scala developer.",
		payMax:      ptr(50000.0),
		payText:     ptr("$50k"),
		postedDate:  time.Now(),
	})

	resp, err := s.Search(context.Background(), &proto.SearchRequest{
		ExactSearchTerms: []string{"scala"},
		PayMin:           100000,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !containsCompany(resp.Jobs, "HighPayCo") {
		t.Error("expected HighPayCo in results")
	}
	if containsCompany(resp.Jobs, "LowPayCo") {
		t.Error("LowPayCo should be excluded by PayMin filter")
	}
}

func TestSearch_MaxApplicants(t *testing.T) {
	s := testServer(t)
	insertJob(t, s.pool, jobSpec{
		title:       "Scala Developer",
		company:     "FewApplicantsCo",
		description: "Scala developer.",
		applicants:  ptr(25),
		postedDate:  time.Now(),
	})
	insertJob(t, s.pool, jobSpec{
		title:       "Scala Developer",
		company:     "ManyApplicantsCo",
		description: "Scala developer.",
		applicants:  ptr(300),
		postedDate:  time.Now(),
	})

	resp, err := s.Search(context.Background(), &proto.SearchRequest{
		ExactSearchTerms: []string{"scala"},
		MaxApplicants:    100,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !containsCompany(resp.Jobs, "FewApplicantsCo") {
		t.Error("expected FewApplicantsCo in results")
	}
	if containsCompany(resp.Jobs, "ManyApplicantsCo") {
		t.Error("ManyApplicantsCo should be excluded by MaxApplicants filter")
	}
}

func TestSearch_MaxApplicants_NullsIncluded(t *testing.T) {
	s := testServer(t)
	insertJob(t, s.pool, jobSpec{
		title:       "Scala Developer",
		company:     "NullApplicantsCo",
		description: "Scala developer.",
		postedDate:  time.Now(),
	})

	resp, err := s.Search(context.Background(), &proto.SearchRequest{
		ExactSearchTerms: []string{"scala"},
		MaxApplicants:    10,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !containsCompany(resp.Jobs, "NullApplicantsCo") {
		t.Error("jobs with no applicant count should be included when MaxApplicants is set")
	}
}

func TestSearch_Days(t *testing.T) {
	s := testServer(t)
	insertJob(t, s.pool, jobSpec{
		title:       "Scala Developer",
		company:     "RecentCo",
		description: "Scala developer.",
		postedDate:  time.Now(),
	})
	insertJob(t, s.pool, jobSpec{
		title:       "Scala Developer",
		company:     "OldCo",
		description: "Scala developer.",
		postedDate:  time.Now().AddDate(0, 0, -10),
	})

	resp, err := s.Search(context.Background(), &proto.SearchRequest{
		ExactSearchTerms: []string{"scala"},
		Days:             3,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !containsCompany(resp.Jobs, "RecentCo") {
		t.Error("expected RecentCo in results")
	}
	if containsCompany(resp.Jobs, "OldCo") {
		t.Error("OldCo should be excluded by Days filter")
	}
}

func TestSearch_DescriptionExclusions(t *testing.T) {
	s := testServer(t)
	insertJob(t, s.pool, jobSpec{
		title:       "Scala Developer",
		company:     "ManagementCo",
		description: "Scala developer who will manage a team.",
		postedDate:  time.Now(),
	})
	insertJob(t, s.pool, jobSpec{
		title:       "Scala Developer",
		company:     "TechOnlyCo",
		description: "Scala developer, purely technical individual contributor role.",
		postedDate:  time.Now(),
	})

	resp, err := s.Search(context.Background(), &proto.SearchRequest{
		ExactSearchTerms:      []string{"scala"},
		DescriptionExclusions: []string{"management"},
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if containsCompany(resp.Jobs, "ManagementCo") {
		t.Error("ManagementCo should be excluded by description exclusion")
	}
	if !containsCompany(resp.Jobs, "TechOnlyCo") {
		t.Error("expected TechOnlyCo in results")
	}
}
