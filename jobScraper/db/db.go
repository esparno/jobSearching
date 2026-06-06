package db

import (
	"context"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect opens a connection pool using DATABASE_URL from the environment.
func Connect(ctx context.Context) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
}
