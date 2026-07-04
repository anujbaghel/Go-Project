package backend

import "context"

type WalletBackend interface {
	EnsureWallet(ctx context.Context, uid int64) error
	DEBIT(ctx context.Context, uid, amount int64, constraintId string) error
	CREDIT(ctx context.Context, uid, amount int64, constraintId string) error
}
