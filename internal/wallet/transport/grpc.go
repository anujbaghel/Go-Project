package transport

import (
	walletservice "Go-project/internal/wallet/service"
	"Go-project/internal/wallet/walletpb"
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type WalletGRPCServer struct {
	walletpb.UnimplementedWalletServiceServer
	wallet *walletservice.Wallet
}

func NewWalletGRPCServer(wallet *walletservice.Wallet) *WalletGRPCServer {
	return &WalletGRPCServer{wallet: wallet}
}

func (s *WalletGRPCServer) Debit(ctx context.Context, req *walletpb.TransactionRequest) (*walletpb.TransactionResponse, error) {
	//Validation
	if req.Amount <= 0 || req.ConstraintId == "" {
		return nil, status.Error(codes.InvalidArgument, "Invalid Request params")
	}

	if err := s.wallet.EnsureWallet(ctx, req.Uid); err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid user wallet does not exist")
	}

	if err := s.wallet.DEBIT(ctx, req.Uid, req.Amount, req.ConstraintId); err != nil {
		if errors.Is(err, walletservice.ErrInsufficientFunds) {
			return nil, status.Error(codes.FailedPrecondition, "Insufficient funds")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &walletpb.TransactionResponse{Status: "OK"}, nil
}

func (s *WalletGRPCServer) Credit(ctx context.Context, req *walletpb.TransactionRequest) (*walletpb.TransactionResponse, error) {

	if req.Amount <= 0 || req.ConstraintId == "" {
		return nil, status.Error(codes.InvalidArgument, "Invalid request params")
	}

	if err := s.wallet.EnsureWallet(ctx, req.Uid); err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid user wallet does not exist")
	}

	if err := s.wallet.CREDIT(ctx, req.Uid, req.Amount, req.ConstraintId); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &walletpb.TransactionResponse{Status: "OK"}, nil
}
