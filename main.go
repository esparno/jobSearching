package main

import (
	"context"
	"jobSearching/api/linkedin"
	"jobSearching/db"
	"log"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	pool, err := db.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	linkedin.ScrapeJobs(100, 0, ctx, pool)
}
