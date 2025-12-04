package middleware

import (
	"context"
	"log/slog"
	"math/rand"
	"net/http"
	"time"
)

type ctxKey string

const (
	CtxKeyRequestID ctxKey = "request_id"
	CtxKeyTraceID   ctxKey = "trace_id"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func generateID() string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	const length = 16

	b := make([]byte, length)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func WithRequestAndTrace(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = generateID()
		}

		traceID := r.Header.Get("X-Trace-ID")
		if traceID == "" {
			traceID = generateID()
		}

		ctx := context.WithValue(r.Context(), CtxKeyRequestID, reqID)
		ctx = context.WithValue(ctx, CtxKeyTraceID, traceID)

		r = r.WithContext(ctx)

		slog.Default().Info("incoming request",
			"request_id", reqID,
			"trace_id", traceID,
			"method", r.Method,
			"path", r.URL.Path,
		)

		next.ServeHTTP(w, r)

		slog.Default().Info("finished request",
			"request_id", reqID,
			"trace_id", traceID,
			"method", r.Method,
			"path", r.URL.Path,
		)
	})
}

func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(CtxKeyRequestID).(string); ok {
		return v
	}
	return ""
}

func TraceIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(CtxKeyTraceID).(string); ok {
		return v
	}
	return ""
}
