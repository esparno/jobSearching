package main

import (
	"context"
	"jobSearching/api/linkedin"
	"jobSearching/db"
	"log"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load(".env.local")

	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	if err := linkedin.Configure(); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	pool, err := db.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	linkedin.ScrapeJobs(ctx, cfg.numJobs, cfg.opts, pool)
}
