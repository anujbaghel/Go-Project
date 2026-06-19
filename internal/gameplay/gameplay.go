package gameplay

import (
	"context"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Service struct {
	pool *pgxpool.Pool
	rdb  *redis.Client
}

func New(pool *pgxpool.Pool, rdb *redis.Client) *Service {
	return &Service{pool: pool, rdb: rdb}
}

func leaderboardKey(ltid int64) string { return fmt.Sprintf("lb:{%d}", ltid) }

type Entry struct {
	Rank  int   `json:"rank"`
	UID   int64 `json:"uid"`
	Score int   `json:"score"`
}

func (s *Service) RecordScore(ctx context.Context, ltid int64, uid int64, score int) error {
	// 1) live leaderboard: sorted set, member = uid, score = score
	if err := s.rdb.ZAdd(ctx, leaderboardKey(ltid), redis.Z{Score: float64(score), Member: uid}).Err(); err != nil {
		return err
	}
	// keep the player's best score on their registeration row
	_, err := s.pool.Exec(ctx,
		`UPDATE registrations SET score = GREATEST(score,$1) WHERE ltid=$2 AND uid=$3`,
		score, ltid, uid)
	return err
}

// TopN reads the top N from the Redis sorted set (highest score = rank 1).
func (s *Service) TopN(ctx context.Context, ltid int64, n int) ([]Entry, error) {
	rows, err := s.rdb.ZRevRangeWithScores(ctx, leaderboardKey(ltid), 0, int64(n-1)).Result()
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(rows))
	for i, z := range rows {
		member, ok := z.Member.(string)
		if !ok {
			continue
		}
		uid, _ := strconv.ParseInt(member, 10, 64)
		// go-redis stores members as string
		out = append(out, Entry{Rank: i + 1, UID: uid, Score: (int(z.Score))})
	}
	return out, nil
}
