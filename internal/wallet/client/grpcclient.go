package client

import (
	walletcontract "Go-project/internal/wallet/contract"
	"Go-project/internal/wallet/walletpb"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/sony/gobreaker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type GrpcClient struct {
	// Add fields for gRPC connection, client, etc.
	rpc walletpb.WalletServiceClient
	cb  *gobreaker.CircuitBreaker
}

func NewGrpcClient(addr string) (*GrpcClient, error) {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "wallet",
		Timeout:     5 * time.Second,
		MaxRequests: 1,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 3
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			slog.Info("wallet breaker", "from", from.String(), "to", to.String())
		},
	})

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &GrpcClient{rpc: walletpb.NewWalletServiceClient(conn), cb: cb}, nil
}

func (c *GrpcClient) EnsureWallet(ctx context.Context, uid int64) error {
	return nil
}

func (c *GrpcClient) DEBIT(ctx context.Context, uid, amount int64, constraintId string) error {
	req := &walletpb.TransactionRequest{
		Uid:          uid,
		Amount:       amount,
		ConstraintId: constraintId,
	}

	return c.callWithRetry(ctx, func(callCtx context.Context) error {
		_, err := c.rpc.Credit(callCtx, req)
		return err
	})
}

func (c *GrpcClient) CREDIT(ctx context.Context, uid, amount int64, constraintId string) error {
	req := &walletpb.TransactionRequest{
		Uid:          uid,
		Amount:       amount,
		ConstraintId: constraintId,
	}
	return c.callWithRetry(ctx, func(callCtx context.Context) error {
		_, err := c.rpc.Credit(callCtx, req)
		return err
	})
}

func (c *GrpcClient) callWithRetry(ctx context.Context, invoke func(context.Context) error) error {
	var lastErr error
	for attempt := 1; attempt < maxAttempts; attempt++ {
		err := c.callOnce(ctx, invoke)
		if err == nil {
			return nil
		}
		if !errors.Is(err, walletcontract.ErrWalletUnavailable) {
			return err // insufficient funds / invalid / unknown → deterministic, don't retry
		}
		lastErr = err // transient - retry below

		if attempt < maxAttempts {
			select {
			case <-ctx.Done():
				return walletcontract.ErrWalletUnavailable
			case <-time.After(retryBackoff * time.Duration(1<<(attempt-1))): // exponential backoff
			}
		}
	}

	slog.Warn("wallet grpc call failed after retries", "err", lastErr)
	return lastErr
}

func (c *GrpcClient) callOnce(ctx context.Context, invoke func(context.Context) error) error {
	res, err := c.cb.Execute(func() (interface{}, error) {
		callCtx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
		defer cancel()

		rerr := invoke(callCtx)
		if rerr == nil {
			return "Ok", nil
		}

		switch status.Code(rerr) {
		case codes.FailedPrecondition:
			return "insufficient", nil
		case codes.InvalidArgument:
			return "invalid", nil
		default:
			return nil, rerr
		}
	})

	if err != nil {
		// Breaker open -> fail fast as unavailable; no point retrying right now.
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			return walletcontract.ErrWalletUnavailable
		}
		return fmt.Errorf("wallet call failed: %w : %v", walletcontract.ErrWalletUnavailable, err)
	}

	switch res {
	case "insufficient":
		return walletcontract.ErrInsufficientFunds
	case "invalid":
		return errors.New("walletclient: invalid request params")
	default:
		return nil
	}
}
