package walletclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Go-project/internal/walletcontract"
)

const testSecret = "test-secret"

func TestDebitOK(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL, testSecret)
	if err := c.DEBIT(context.Background(), 1, 100, "c1"); err != nil {
		t.Fatalf("DEBIT: want nil, got %v", err)
	}
	if gotAuth != "Bearer "+testSecret {
		t.Fatalf("auth header: want %q, got %q", "Bearer "+testSecret, gotAuth)
	}
}

func TestDebitInsufficient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
	}))
	defer srv.Close()

	c := New(srv.URL, testSecret)
	err := c.DEBIT(context.Background(), 1, 100, "c1")
	if !errors.Is(err, walletcontract.ErrInsufficientFunds) {
		t.Fatalf("want ErrInsufficientFunds, got %v", err)
	}
}

func TestDebitServerErrorBecomesUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(srv.URL, testSecret)
	err := c.DEBIT(context.Background(), 1, 100, "c1")
	if !errors.Is(err, walletcontract.ErrWalletUnavailable) {
		t.Fatalf("want ErrWalletUnavailable on 5xx, got %v", err)
	}
}

func TestCreditOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != walletcontract.PathCredit {
			t.Errorf("path: want %q, got %q", walletcontract.PathCredit, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL, testSecret)
	if err := c.CREDIT(context.Background(), 1, 50, "c2"); err != nil {
		t.Fatalf("CREDIT: want nil, got %v", err)
	}
}
