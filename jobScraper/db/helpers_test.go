package db

import (
	"context"
	"fmt"
	"jobSearching/models"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func TestMain(m *testing.M) {
	_ = godotenv.Load("../../.env.test")
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

func testSourceID(t *testing.T) string {
	return fmt.Sprintf("%s-%d", t.Name(), time.Now().UnixNano())
}

func testRunID(t *testing.T) string {
	return uuid.New().String()
}

func testJob(sourceID string) models.Job {
	return models.Job{
		Source:     models.LinkedIn,
		SourceID:   sourceID,
		Title:      "Software Engineer",
		Company:    "Acme Corp",
		Location:   "Remote",
		URL:        "https://example.com/jobs/" + sourceID,
		PostedDate: "2026-05-26",
	}
}