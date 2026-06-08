package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// MustConnect returns a connection pool or dies trying. A pool (not a single
// connection) is what lets many goroutines hit the DB concurrently.
func MustConnect(ctx context.Context, url string) *pgxpool.Pool {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		panic(err)
	}
	if err := pool.Ping(ctx); err != nil {
		panic(err)
	}
	return pool
}
