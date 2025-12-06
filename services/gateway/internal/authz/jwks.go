package authz

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"gateway/internal/middleware"
	"gateway/internal/observability/metrics"
	obsmw "gateway/internal/observability/middleware"

	"github.com/MicahParks/keyfunc"
	"github.com/golang-jwt/jwt/v4"
)

type JWTValidator struct {
	jwks   *keyfunc.JWKS
	issuer string
}

func NewJWTValidator(ctx context.Context, jwksURL, issuer string) (*JWTValidator, error) {
	options := keyfunc.Options{
		RefreshInterval:   time.Minute * 15,
		RefreshTimeout:    time.Second * 10,
		RefreshUnknownKID: true,
	}
	jwks, err := keyfunc.Get(jwksURL, options)
	if err != nil {
		return nil, err
	}
	return &JWTValidator{jwks: jwks, issuer: issuer}, nil
}

func (j *JWTValidator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := "success"
		defer func() {
			metrics.AuthenticationAttemptsTotal.WithLabelValues("jwks", result).Inc()
		}()
		reqID := obsmw.RequestIDFromContext(r.Context())
		traceID := obsmw.TraceIDFromContext(r.Context())
		raw := r.Header.Get("Authorization")
		if !strings.HasPrefix(strings.ToLower(raw), "bearer ") {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			result = "failure"
			slog.Warn("gateway jwks missing bearer", "request_id", reqID, "trace_id", traceID)
			return
		}
		tokStr := strings.TrimSpace(raw[len("Bearer "):])

		token, err := jwt.Parse(tokStr, j.jwks.Keyfunc)
		if err != nil || !token.Valid {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			result = "failure"
			slog.Warn("gateway jwks invalid token", "error", err, "request_id", reqID, "trace_id", traceID)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			result = "failure"
			http.Error(w, "invalid token claims", http.StatusUnauthorized)
			return
		}
		// optional issuer check
		if iss, _ := claims["iss"].(string); iss != "" && iss != j.issuer {
			result = "failure"
			http.Error(w, "issuer mismatch", http.StatusUnauthorized)
			slog.Warn("gateway jwks issuer mismatch", "issuer", iss, "request_id", reqID, "trace_id", traceID)
			return
		}
		sub, _ := claims["sub"].(string)
		if sub == "" {
			result = "failure"
			http.Error(w, "no subject", http.StatusUnauthorized)
			slog.Warn("gateway jwks missing subject", "request_id", reqID, "trace_id", traceID)
			return
		}
		ctx := middleware.WithSubject(r.Context(), sub)
		slog.Info("gateway auth passed", "method", "jwks", "subject", sub, "request_id", reqID, "trace_id", traceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Alias for easier import in main.go
func (j *JWTValidator) Handler() func(http.Handler) http.Handler { return j.Middleware }
