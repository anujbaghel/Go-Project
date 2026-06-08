package gateway

import (
	"context"
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
			if raw == "" {
				http.Error(w, "Missing Token: Unathorized Access", http.StatusUnauthorized)
				return
			}

			token, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
				// SECURITY: pin the algorithm. Without this check an attacker
				// could send alg=none or a different alg and bypass the signature.
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return secret, nil // the key used to VERIFY the signature
			})
			if err != nil || !token.Valid {
				http.Error(w, "Invalid Token", http.StatusUnauthorized)
				return
			}
			claims := token.Claims.(jwt.MapClaims)
			uidFloat, ok := claims["uid"].(float64)
			if !ok {
				http.Error(w, "Invalid Claims", http.StatusUnauthorized)
				return
			}
			uid := int64(uidFloat)

			// Attach a new context and pass the request down the chain
			ctx := context.WithValue(req.Context(), userIDKey, uid)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	}
}
