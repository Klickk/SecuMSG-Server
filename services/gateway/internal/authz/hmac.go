package authz

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"gateway/internal/observability/metrics"
	obsmw "gateway/internal/observability/middleware"

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
		result := "success"
		defer metrics.AuthenticationAttemptsTotal.WithLabelValues("hmac", result).Inc()
		reqID := obsmw.RequestIDFromContext(r.Context())
		traceID := obsmw.TraceIDFromContext(r.Context())
		raw := r.Header.Get("Authorization")
		if !strings.HasPrefix(strings.ToLower(raw), "bearer ") {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			result = "failure"
			slog.Warn("gateway auth missing bearer", "request_id", reqID, "trace_id", traceID)
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
			result = "failure"
			http.Error(w, "invalid token", http.StatusUnauthorized)
			slog.Warn("gateway auth invalid token", "error", err, "request_id", reqID, "trace_id", traceID)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			result = "failure"
			http.Error(w, "invalid token claims", http.StatusUnauthorized)
			slog.Warn("gateway auth invalid claims", "request_id", reqID, "trace_id", traceID)
			return
		}
		if iss, _ := claims["iss"].(string); iss != "" && iss != h.issuer {
			result = "failure"
			http.Error(w, "issuer mismatch", http.StatusUnauthorized)
			slog.Warn("gateway auth issuer mismatch", "issuer", iss, "request_id", reqID, "trace_id", traceID)
			return
		}
		sub, _ := claims["sub"].(string)
		if sub == "" {
			result = "failure"
			http.Error(w, "no subject", http.StatusUnauthorized)
			slog.Warn("gateway auth missing subject", "request_id", reqID, "trace_id", traceID)
			return
		}

		// store sub in context (local key to avoid pkg cycles)
		ctx := r.Context()
		ctx = contextWithSubject(ctx, sub)
		slog.Info("gateway auth passed", "method", "hmac", "subject", sub, "request_id", reqID, "trace_id", traceID)
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
