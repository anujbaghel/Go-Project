package redisx

import (
	"context"

	"github.com/redis/go-redis/v9"
)

func MustConnect(ctx context.Context, addr string) *redis.Client {
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	if err := rdb.Ping(ctx).Err(); err != nil {
		panic(err)
	}
	return rdb
}
