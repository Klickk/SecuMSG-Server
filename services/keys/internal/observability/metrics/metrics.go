package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"service", "method", "path", "status"},
	)

	HTTPRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "method", "path"},
	)
)

func MustRegister(serviceName string) {
	HTTPRequestsTotal = HTTPRequestsTotal.MustCurryWith(prometheus.Labels{"service": serviceName})
	HTTPRequestDurationSeconds = HTTPRequestDurationSeconds.MustCurryWith(prometheus.Labels{"service": serviceName})

	prometheus.MustRegister(
		HTTPRequestsTotal,
		HTTPRequestDurationSeconds,
	)
}
