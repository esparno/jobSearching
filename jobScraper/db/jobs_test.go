package db

import (
	"context"
	"jobSearching/models"
	"testing"
	"time"
)

func TestNullableString(t *testing.T) {
	if got := nullableString("hello"); got == nil || *got != "hello" {
		t.Errorf("non-empty: got %v, want %q", got, "hello")
	}
	if got := nullableString(""); got != nil {
		t.Errorf("empty: got %v, want nil", got)
	}
}

func TestUpsertJob_NewJob(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	sourceID := testSourceID(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM jobs WHERE source_id = $1", sourceID)
	})

	id, isNew, err := UpsertJob(ctx, pool, testJob(sourceID))
	if err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}
	if !isNew {
		t.Error("isNew: got false, want true for first insert")
	}
	if id <= 0 {
		t.Errorf("id: got %d, want > 0", id)
	}
}

func TestUpsertJob_Duplicate(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	sourceID := testSourceID(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM jobs WHERE source_id = $1", sourceID)
	})

	job := testJob(sourceID)
	id1, isNew1, err := UpsertJob(ctx, pool, job)
	if err != nil {
		t.Fatalf("first UpsertJob: %v", err)
	}
	if !isNew1 {
		t.Error("first insert: expected isNew=true")
	}

	id2, isNew2, err := UpsertJob(ctx, pool, job)
	if err != nil {
		t.Fatalf("second UpsertJob: %v", err)
	}
	if isNew2 {
		t.Error("second insert: expected isNew=false")
	}
	if id1 != id2 {
		t.Errorf("id: got %d on second insert, want %d (same as first)", id2, id1)
	}
}

func TestUpsertJob_UpdatesLastSeen(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	sourceID := testSourceID(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM jobs WHERE source_id = $1", sourceID)
	})

	job := testJob(sourceID)
	if _, _, err := UpsertJob(ctx, pool, job); err != nil {
		t.Fatalf("first UpsertJob: %v", err)
	}

	var lastSeen1 time.Time
	_ = pool.QueryRow(ctx, "SELECT last_seen FROM jobs WHERE source_id = $1", sourceID).Scan(&lastSeen1)

	time.Sleep(10 * time.Millisecond)
	if _, _, err := UpsertJob(ctx, pool, job); err != nil {
		t.Fatalf("second UpsertJob: %v", err)
	}

	var lastSeen2 time.Time
	_ = pool.QueryRow(ctx, "SELECT last_seen FROM jobs WHERE source_id = $1", sourceID).Scan(&lastSeen2)

	if !lastSeen2.After(lastSeen1) {
		t.Errorf("last_seen: expected update on second upsert, got %v → %v", lastSeen1, lastSeen2)
	}
}

func TestInsertJobDetail(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	sourceID := testSourceID(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM job_details WHERE source_id = $1", sourceID)
		_, _ = pool.Exec(ctx, "DELETE FROM jobs WHERE source_id = $1", sourceID)
	})

	job := testJob(sourceID)
	id, _, err := UpsertJob(ctx, pool, job)
	if err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}
	job.ID = id

	payMin, payMax := 120000.0, 160000.0
	applicantsCount := 53
	detail := models.JobDetail{
		SourceID:            sourceID,
		Seniority:           "Mid-Senior level",
		EmploymentType:      "Full-time",
		WorkType:            "Remote",
		Description:         "Build great things.",
		ApplicantsText:      "53 applicants",
		Applicants:          &applicantsCount,
		ApplicantsQualifier: models.ApplicantsEqual,
		PayType:             models.PayTypeSalary,
		PayMin:              &payMin,
		PayMax:              &payMax,
		PayText:             "$120,000 - $160,000",
	}

	if err := InsertJobDetail(ctx, pool, job, detail); err != nil {
		t.Fatalf("InsertJobDetail: %v", err)
	}

	var storedDesc, storedPayText, storedApplicantsText, storedApplicantsQualifier string
	var storedPayMin, storedPayMax float64
	var storedApplicants int
	err = pool.QueryRow(ctx, `
		SELECT description, pay_text, pay_min, pay_max, applicants_text, applicants, applicants_qualifier
		FROM job_details WHERE source_id = $1
	`, sourceID).Scan(&storedDesc, &storedPayText, &storedPayMin, &storedPayMax, &storedApplicantsText, &storedApplicants, &storedApplicantsQualifier)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if storedDesc != detail.Description {
		t.Errorf("description: got %q, want %q", storedDesc, detail.Description)
	}
	if storedPayText != detail.PayText {
		t.Errorf("pay_text: got %q, want %q", storedPayText, detail.PayText)
	}
	if storedPayMin != payMin {
		t.Errorf("pay_min: got %v, want %v", storedPayMin, payMin)
	}
	if storedPayMax != payMax {
		t.Errorf("pay_max: got %v, want %v", storedPayMax, payMax)
	}
	if storedApplicantsText != detail.ApplicantsText {
		t.Errorf("applicants_text: got %q, want %q", storedApplicantsText, detail.ApplicantsText)
	}
	if storedApplicants != applicantsCount {
		t.Errorf("applicants: got %d, want %d", storedApplicants, applicantsCount)
	}
	if storedApplicantsQualifier != string(detail.ApplicantsQualifier) {
		t.Errorf("applicants_qualifier: got %q, want %q", storedApplicantsQualifier, detail.ApplicantsQualifier)
	}
}

func TestInsertJobDetail_SkipsApplicantsWithinSameRun(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	sourceID := testSourceID(t)
	runID := testRunID(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM job_details WHERE source_id = $1", sourceID)
		_, _ = pool.Exec(ctx, "DELETE FROM jobs WHERE source_id = $1", sourceID)
	})

	job := testJob(sourceID)
	job.RunID = runID
	id, _, err := UpsertJob(ctx, pool, job)
	if err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}
	job.ID = id

	firstCount := 53
	first := models.JobDetail{
		SourceID:            sourceID,
		Description:         "First",
		ApplicantsText:      "53 applicants",
		Applicants:          &firstCount,
		ApplicantsQualifier: models.ApplicantsEqual,
	}
	if err := InsertJobDetail(ctx, pool, job, first); err != nil {
		t.Fatalf("first InsertJobDetail: %v", err)
	}

	secondCount := 100
	second := models.JobDetail{
		SourceID:            sourceID,
		Description:         "Second",
		ApplicantsText:      "100 applicants",
		Applicants:          &secondCount,
		ApplicantsQualifier: models.ApplicantsEqual,
	}
	if err := InsertJobDetail(ctx, pool, job, second); err != nil {
		t.Fatalf("second InsertJobDetail: %v", err)
	}

	var desc string
	var applicants int
	_ = pool.QueryRow(ctx, "SELECT description, applicants FROM job_details WHERE source_id = $1", sourceID).Scan(&desc, &applicants)
	if desc != "First" {
		t.Errorf("description: got %q, want %q (should be preserved)", desc, "First")
	}
	if applicants != firstCount {
		t.Errorf("applicants: got %d, want %d (should not update within same run)", applicants, firstCount)
	}
}

func TestInsertJobDetail_UpdatesApplicantsOnRerun(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	sourceID := testSourceID(t)
	runID1 := testRunID(t)
	runID2 := testRunID(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM job_details WHERE source_id = $1", sourceID)
		_, _ = pool.Exec(ctx, "DELETE FROM jobs WHERE source_id = $1", sourceID)
	})

	job := testJob(sourceID)
	job.RunID = runID1
	id, _, err := UpsertJob(ctx, pool, job)
	if err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}
	job.ID = id

	firstCount := 53
	first := models.JobDetail{
		SourceID:            sourceID,
		Description:         "First",
		ApplicantsText:      "53 applicants",
		Applicants:          &firstCount,
		ApplicantsQualifier: models.ApplicantsEqual,
	}
	if err := InsertJobDetail(ctx, pool, job, first); err != nil {
		t.Fatalf("first InsertJobDetail: %v", err)
	}

	job.RunID = runID2
	secondCount := 100
	second := models.JobDetail{
		SourceID:            sourceID,
		Description:         "Second",
		ApplicantsText:      "100 applicants",
		Applicants:          &secondCount,
		ApplicantsQualifier: models.ApplicantsEqual,
	}
	if err := InsertJobDetail(ctx, pool, job, second); err != nil {
		t.Fatalf("second InsertJobDetail: %v", err)
	}

	var desc, applicantsText string
	var applicants int
	_ = pool.QueryRow(ctx, `
		SELECT description, applicants, applicants_text FROM job_details WHERE source_id = $1
	`, sourceID).Scan(&desc, &applicants, &applicantsText)
	if desc != "First" {
		t.Errorf("description: got %q, want %q (should be preserved on rerun)", desc, "First")
	}
	if applicants != secondCount {
		t.Errorf("applicants: got %d, want %d (should update on rerun)", applicants, secondCount)
	}
	if applicantsText != second.ApplicantsText {
		t.Errorf("applicants_text: got %q, want %q (should update on rerun)", applicantsText, second.ApplicantsText)
	}
}

func TestInsertJobDetail_SkipsApplicantsWhenAtOrAbove200(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	sourceID := testSourceID(t)
	runID1 := testRunID(t)
	runID2 := testRunID(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM job_details WHERE source_id = $1", sourceID)
		_, _ = pool.Exec(ctx, "DELETE FROM jobs WHERE source_id = $1", sourceID)
	})

	job := testJob(sourceID)
	job.RunID = runID1
	id, _, err := UpsertJob(ctx, pool, job)
	if err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}
	job.ID = id

	firstCount := 200
	first := models.JobDetail{
		SourceID:            sourceID,
		Description:         "First",
		ApplicantsText:      "200 applicants",
		Applicants:          &firstCount,
		ApplicantsQualifier: models.ApplicantsEqual,
	}
	if err := InsertJobDetail(ctx, pool, job, first); err != nil {
		t.Fatalf("first InsertJobDetail: %v", err)
	}

	job.RunID = runID2
	secondCount := 250
	second := models.JobDetail{
		SourceID:            sourceID,
		Description:         "Second",
		ApplicantsText:      "250 applicants",
		Applicants:          &secondCount,
		ApplicantsQualifier: models.ApplicantsEqual,
	}
	if err := InsertJobDetail(ctx, pool, job, second); err != nil {
		t.Fatalf("second InsertJobDetail: %v", err)
	}

	var applicants int
	_ = pool.QueryRow(ctx, "SELECT applicants FROM job_details WHERE source_id = $1", sourceID).Scan(&applicants)
	if applicants != firstCount {
		t.Errorf("applicants: got %d, want %d (should not update when already >= 200)", applicants, firstCount)
	}
}

func TestGetJobsForApplicantUpdate_ExcludesCurrentRunDetails(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	runID := testRunID(t)

	// Job detailed in the current run — should be excluded.
	sameRunID := testSourceID(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM job_details WHERE source_id = $1", sameRunID)
		_, _ = pool.Exec(ctx, "DELETE FROM jobs WHERE source_id = $1", sameRunID)
	})
	job := testJob(sameRunID)
	job.RunID = runID
	id, _, _ := UpsertJob(ctx, pool, job)
	job.ID = id
	count := 50
	_ = InsertJobDetail(ctx, pool, job, models.JobDetail{
		SourceID:   sameRunID,
		Applicants: &count,
	})

	// Job seen again this run but detailed in a previous run — should be included.
	prevRunID := testSourceID(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM job_details WHERE source_id = $1", prevRunID)
		_, _ = pool.Exec(ctx, "DELETE FROM jobs WHERE source_id = $1", prevRunID)
	})
	oldJob := testJob(prevRunID)
	oldJob.RunID = testRunID(t)
	oldID, _, _ := UpsertJob(ctx, pool, oldJob)
	oldJob.ID = oldID
	_ = InsertJobDetail(ctx, pool, oldJob, models.JobDetail{
		SourceID:   prevRunID,
		Applicants: &count,
	})
	oldJob.RunID = runID
	_, _, _ = UpsertJob(ctx, pool, oldJob)

	jobs, err := GetJobsForApplicantUpdateByRunID(ctx, pool, runID)
	if err != nil {
		t.Fatalf("GetJobsForApplicantUpdateByRunID: %v", err)
	}

	for _, j := range jobs {
		if j.SourceID == sameRunID {
			t.Error("job detailed in current run should not be returned for applicant update")
		}
	}
	found := false
	for _, j := range jobs {
		if j.SourceID == prevRunID {
			found = true
		}
	}
	if !found {
		t.Error("job detailed in previous run should be returned for applicant update")
	}
}

func TestInsertJobDetail_NullPayFields(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	sourceID := testSourceID(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM job_details WHERE source_id = $1", sourceID)
		_, _ = pool.Exec(ctx, "DELETE FROM jobs WHERE source_id = $1", sourceID)
	})

	job := testJob(sourceID)
	id, _, err := UpsertJob(ctx, pool, job)
	if err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}
	job.ID = id

	detail := models.JobDetail{SourceID: sourceID, Description: "No pay info"}
	if err := InsertJobDetail(ctx, pool, job, detail); err != nil {
		t.Fatalf("InsertJobDetail: %v", err)
	}

	var payMin, payMax *float64
	var payText *string
	err = pool.QueryRow(ctx, `
		SELECT pay_min, pay_max, pay_text FROM job_details WHERE source_id = $1
	`, sourceID).Scan(&payMin, &payMax, &payText)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if payMin != nil {
		t.Errorf("pay_min: got %v, want nil", *payMin)
	}
	if payMax != nil {
		t.Errorf("pay_max: got %v, want nil", *payMax)
	}
	if payText != nil {
		t.Errorf("pay_text: got %q, want nil", *payText)
	}
}
