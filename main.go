package main

import (
	"context"
	"jobSearching/api/linkedin"
	"jobSearching/db"
	"jobSearching/models"
	"log"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load(".env.local")

	if err := linkedin.Configure(); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	pool, err := db.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	linkedin.ScrapeJobs(ctx, 10, linkedin.SearchOptions{
		Keywords:   linkedin.KeywordsSoftwareEngineer,
		TimePosted: linkedin.OneDay,
		WorkType:   models.Remote,
	}, pool)
}
