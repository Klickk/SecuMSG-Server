package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"keys/internal/observability/metrics"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func WithMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		sr := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(sr, r)

		duration := time.Since(start).Seconds()
		path := r.URL.Path
		method := r.Method
		statusStr := strconv.Itoa(sr.status)

		metrics.HTTPRequestsTotal.WithLabelValues(method, path, statusStr).Inc()
		metrics.HTTPRequestDurationSeconds.WithLabelValues(method, path).Observe(duration)

		slog.Default().Debug("request metrics updated",
			"method", method,
			"path", path,
			"status", sr.status,
			"duration_seconds", duration,
		)
	})
}
