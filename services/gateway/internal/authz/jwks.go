package authz

import (
	"context"
	"net/http"
	"strings"
	"time"

	"gateway/internal/middleware"

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
		raw := r.Header.Get("Authorization")
		if !strings.HasPrefix(strings.ToLower(raw), "bearer ") {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}
		tokStr := strings.TrimSpace(raw[len("Bearer "):])

		token, err := jwt.Parse(tokStr, j.jwks.Keyfunc)
		if err != nil || !token.Valid {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, "invalid token claims", http.StatusUnauthorized)
			return
		}
		// optional issuer check
		if iss, _ := claims["iss"].(string); iss != "" && iss != j.issuer {
			http.Error(w, "issuer mismatch", http.StatusUnauthorized)
			return
		}
		sub, _ := claims["sub"].(string)
		if sub == "" {
			http.Error(w, "no subject", http.StatusUnauthorized)
			return
		}
		ctx := middleware.WithSubject(r.Context(), sub)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Alias for easier import in main.go
func (j *JWTValidator) Handler() func(http.Handler) http.Handler { return j.Middleware }
