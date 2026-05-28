package db

import (
	"context"
	"jobSearching/models"
	"testing"
	"time"
)

// --- nullableString ---

func TestNullableString(t *testing.T) {
	if got := nullableString("hello"); got == nil || *got != "hello" {
		t.Errorf("non-empty: got %v, want %q", got, "hello")
	}
	if got := nullableString(""); got != nil {
		t.Errorf("empty: got %v, want nil", got)
	}
}

// --- UpsertJob ---

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

// --- InsertJobDetail ---

func TestInsertJobDetail(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	sourceID := testSourceID(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM job_details WHERE source_id = $1", sourceID)
		_, _ = pool.Exec(ctx, "DELETE FROM jobs WHERE source_id = $1", sourceID)
	})

	job := testJob(sourceID)
	jobID, _, err := UpsertJob(ctx, pool, job)
	if err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}

	payMin, payMax := 120000.0, 160000.0
	applicantsCount := 53
	detail := models.JobDetail{
		SourceID:       sourceID,
		Seniority:      "Mid-Senior level",
		EmploymentType: "Full-time",
		WorkType:       "Remote",
		Description:    "Build great things.",
		ApplicantsText: "53 applicants",
		Applicants:     &applicantsCount,
		PayType:        models.PayTypeSalary,
		PayMin:         &payMin,
		PayMax:         &payMax,
		PayText:        "$120,000 - $160,000",
	}

	if err := InsertJobDetail(ctx, pool, jobID, job, detail); err != nil {
		t.Fatalf("InsertJobDetail: %v", err)
	}

	var storedDesc, storedPayText, storedApplicantsText string
	var storedPayMin, storedPayMax float64
	var storedApplicants int
	err = pool.QueryRow(ctx, `
		SELECT description, pay_text, pay_min, pay_max, applicants_text, applicants
		FROM job_details WHERE source_id = $1
	`, sourceID).Scan(&storedDesc, &storedPayText, &storedPayMin, &storedPayMax, &storedApplicantsText, &storedApplicants)
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
}

func TestInsertJobDetail_Idempotent(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	sourceID := testSourceID(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM job_details WHERE source_id = $1", sourceID)
		_, _ = pool.Exec(ctx, "DELETE FROM jobs WHERE source_id = $1", sourceID)
	})

	job := testJob(sourceID)
	jobID, _, err := UpsertJob(ctx, pool, job)
	if err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}

	first := models.JobDetail{SourceID: sourceID, Description: "First"}
	if err := InsertJobDetail(ctx, pool, jobID, job, first); err != nil {
		t.Fatalf("first InsertJobDetail: %v", err)
	}

	// Second insert should be silently ignored (ON CONFLICT DO NOTHING).
	second := models.JobDetail{SourceID: sourceID, Description: "Second"}
	if err := InsertJobDetail(ctx, pool, jobID, job, second); err != nil {
		t.Fatalf("second InsertJobDetail: %v", err)
	}

	var desc string
	_ = pool.QueryRow(ctx, "SELECT description FROM job_details WHERE source_id = $1", sourceID).Scan(&desc)
	if desc != "First" {
		t.Errorf("description: got %q, want %q (first insert should be preserved)", desc, "First")
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
	jobID, _, err := UpsertJob(ctx, pool, job)
	if err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}

	detail := models.JobDetail{SourceID: sourceID, Description: "No pay info"}
	if err := InsertJobDetail(ctx, pool, jobID, job, detail); err != nil {
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
