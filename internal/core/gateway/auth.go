package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type ctxKey string

const userIDKey ctxKey = "uid"

func UserID(ctx context.Context) (int64, bool) {
	uid, ok := ctx.Value(userIDKey).(int64)
	return uid, ok
}

// Token Issue
func IssueToken(secret []byte, uid int64) (string, error) {
	claims := jwt.MapClaims{
		"uid": uid,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(secret) // HMAC-SHA256 over header.payload using secret
}

func Auth(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			raw := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
			uid, err := ParseToken(secret, raw)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			// Attach a new context and pass the request down the chain
			ctx := context.WithValue(req.Context(), userIDKey, uid)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	}
}

func ParseToken(secret []byte, raw string) (int64, error) {
	if raw == "" {
		return 0, errors.New("Empty Token")
	}
	token, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil || !token.Valid {
		return 0, errors.New("invalid token")
	}
	uidFloat, ok := token.Claims.(jwt.MapClaims)["uid"].(float64)
	if !ok {
		return 0, errors.New("invalid claims")
	}
	return int64(uidFloat), nil
}
