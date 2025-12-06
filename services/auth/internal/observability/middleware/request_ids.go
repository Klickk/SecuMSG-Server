package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

type ctxKey string

const (
	CtxKeyRequestID ctxKey = "request_id"
	CtxKeyTraceID   ctxKey = "trace_id"
)

func generateID() string {
	buf := make([]byte, 8) // 16 hex chars
	if _, err := rand.Read(buf); err == nil {
		return hex.EncodeToString(buf)
	}
	// Fallback is monotonic-ish; keeps IDs non-empty even if entropy unavailable.
	return strconv.FormatInt(time.Now().UnixNano(), 36)
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
