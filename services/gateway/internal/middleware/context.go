package middleware

import (
	"context"
	"net/http"

	obsmw "gateway/internal/observability/middleware"

	"github.com/google/uuid"
)

type CtxSubKey struct{}

func PropagateRequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			id := obsmw.RequestIDFromContext(r.Context())
			if id == "" {
				id = r.Header.Get("X-Request-ID")
			}
			if id == "" {
				id = uuid.NewString()
			}
			w.Header().Set("X-Request-ID", id)

			traceID := obsmw.TraceIDFromContext(r.Context())
			if traceID != "" {
				w.Header().Set("X-Trace-ID", traceID)
			}
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

// helper to set subject after JWT validation
func WithSubject(ctx context.Context, sub string) context.Context {
	return context.WithValue(ctx, CtxSubKey{}, sub)
}
