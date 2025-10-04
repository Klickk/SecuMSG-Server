package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type CtxSubKey struct{}

func PropagateRequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-Id")
			if id == "" {
				id = uuid.NewString()
			}
			w.Header().Set("X-Request-Id", id)
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

// helper to set subject after JWT validation
func WithSubject(ctx context.Context, sub string) context.Context {
	return context.WithValue(ctx, CtxSubKey{}, sub)
}
