package main

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

var jobIDCounter atomic.Int64

func TestMain(m *testing.M) {
	_ = godotenv.Load("../.env.test")
	os.Exit(m.Run())
}

func connectTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func testServer(t *testing.T) *server {
	t.Helper()
	return &server{pool: connectTestDB(t)}
}

type jobSpec struct {
	title       string
	company     string
	description string
	payMin      *float64
	payMax      *float64
	payText     *string
	applicants  *int
	postedDate  time.Time
}

func insertJob(t *testing.T, pool *pgxpool.Pool, spec jobSpec) {
	t.Helper()
	ctx := context.Background()
	sourceID := fmt.Sprintf("test-%d", jobIDCounter.Add(1))

	var jobID int64
	err := pool.QueryRow(ctx, `
		INSERT INTO jobs (source, source_id, title, company, url, posted_date, run_id)
		VALUES ('test', $1, $2, $3, '', $4, 'test-run')
		RETURNING id`,
		sourceID, spec.title, spec.company, spec.postedDate.Format("2006-01-02"),
	).Scan(&jobID)
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO job_details (job_id, source, source_id, description, pay_min, pay_max, pay_text, applicants, run_id)
		VALUES ($1, 'test', $2, $3, $4, $5, $6, $7, 'test-run')`,
		jobID, sourceID, spec.description, spec.payMin, spec.payMax, spec.payText, spec.applicants,
	)
	if err != nil {
		t.Fatalf("insert job_details: %v", err)
	}

	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM job_details WHERE job_id = $1", jobID)
		pool.Exec(ctx, "DELETE FROM jobs WHERE id = $1", jobID)
	})
}

func ptr[T any](v T) *T { return &v }