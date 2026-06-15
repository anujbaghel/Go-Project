package tournament

import (
	"Go-project/internal/wallet"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var validNext = map[string]map[string]bool{
	"OPEN":      {"RUNNING": true, "CANCELLED": true},
	"RUNNING":   {"RESULT": true, "CANCELLED": true},
	"RESULT":    {},
	"CANCELLED": {},
}

var (
	ErrNotFound           = errors.New("tournament not found")
	ErrRegistrationClosed = errors.New("registration closed")
)

type Tournament struct {
	Ltid     int64
	EntryFee int64
	Name     string
	Status   string
}

type Service struct {
	pool   *pgxpool.Pool
	wallet *wallet.Wallet
}

func NEW(p *pgxpool.Pool, w *wallet.Wallet) *Service {
	return &Service{pool: p, wallet: w}
}

func (s *Service) CREATE(ctx context.Context, name string, entryFee int64, instantMinScore *int) (int64, error) {
	isIEFREnabled := instantMinScore != nil && *instantMinScore > 0
	var ltid int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO tournaments (name, entry_fee, instant_min_score, is_instant_efc)
	 VALUES ($1, $2, $3, $4) RETURNING ltid`, name, entryFee, *instantMinScore, isIEFREnabled).Scan(&ltid)

	return ltid, err
}

func (s *Service) GetTournament(ctx context.Context, ltid int64) (Tournament, error) {
	var t Tournament
	err := s.pool.QueryRow(ctx,
		`SELECT ltid, name, entry_fee, status FROM tournaments WHERE ltid=$1`, ltid).Scan(&t.Ltid, &t.Name, &t.EntryFee, &t.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, err
}

func (s *Service) Register(ctx context.Context, ltid int64, uid int64) error {
	t, err := s.GetTournament(ctx, ltid)
	if err != nil {
		return err
	}

	if t.Status != "OPEN" {
		return ErrRegistrationClosed
	}

	// Take money first, constraint_id is derived from immutable ids -> retry-safe
	constraintId := fmt.Sprintf("reg:%d:%d", ltid, uid)
	if err := s.wallet.DEBIT(ctx, uid, t.EntryFee, constraintId); err != nil {
		return err
	}

	// Record the registeration post debit. On Conflict do nothing -> re-returning is harmless
	_, err = s.pool.Exec(ctx,
		`INSERT INTO registrations (ltid, uid, entry_fee, bucket_slot) 
	VALUES ($1, $2, $3, $4)
	ON CONFLICT (ltid, uid) DO NOTHING`, ltid, uid, t.EntryFee, uid%1024)
	return err
}

func (s *Service) transition(ctx context.Context, ltid int64, from, to string) error {
	if !validNext[from][to] {
		return fmt.Errorf("invalid transition %s -> %s", from, to)
	}

	tag, err := s.pool.Exec(ctx,
		`UPDATE tournaments
	SET status=$1 WHERE ltid=$2 AND status=$3`, to, ltid, from)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// either tournament does not exist or it already moved On
		return fmt.Errorf("tournament %d not in expected state %s", ltid, from)
	}
	return nil
}

func (s *Service) Start(ctx context.Context, ltid int64) error {
	return s.transition(ctx, ltid, "OPEN", "RUNNING")
}

func (s *Service) DeclareResult(ctx context.Context, ltid int64) error {
	return s.transition(ctx, ltid, "RUNNING", "RESULT")
}

func (s *Service) Cancel(ctx context.Context, ltid int64) error {
	// Cancel from OPEN or Running
	tag, err := s.pool.Exec(ctx,
		`UPDATE tournaments SET status='CANCELLED' WHERE ltid=$1 AND status IN ('OPEN','RUNNING')`, ltid)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("tournament %d cannot be cancelled from its current state", ltid)
	}
	return nil
}
