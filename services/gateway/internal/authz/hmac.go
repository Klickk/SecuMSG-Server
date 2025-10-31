package authz

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type HMACValidator struct {
	secret []byte
	issuer string
}

func NewHMACValidator(secret, issuer string) *HMACValidator {
	return &HMACValidator{
		secret: []byte(secret),
		issuer: issuer,
	}
}

func (h *HMACValidator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.Header.Get("Authorization")
		if !strings.HasPrefix(strings.ToLower(raw), "bearer ") {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}
		tokStr := strings.TrimSpace(raw[len("Bearer "):])

		token, err := jwt.Parse(tokStr, func(token *jwt.Token) (interface{}, error) {
			// Ensure HS* (HMAC) only
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %T", token.Method)
			}
			return h.secret, nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, "invalid token claims", http.StatusUnauthorized)
			return
		}
		if iss, _ := claims["iss"].(string); iss != "" && iss != h.issuer {
			http.Error(w, "issuer mismatch", http.StatusUnauthorized)
			return
		}
		sub, _ := claims["sub"].(string)
		if sub == "" {
			http.Error(w, "no subject", http.StatusUnauthorized)
			return
		}

		// store sub in context (local key to avoid pkg cycles)
		ctx := r.Context()
		ctx = contextWithSubject(ctx, sub)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Small local helpers so we don't import another package
type subjectKey struct{}

func contextWithSubject(ctx context.Context, sub string) context.Context {
	return context.WithValue(ctx, subjectKey{}, sub)
}

func SubjectFrom(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(subjectKey{}).(string)
	return v, ok
}
