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

	AuthRegistrationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_registrations_total",
			Help: "Total number of registration attempts.",
		},
		[]string{"service", "result"},
	)

	AuthLoginsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_logins_total",
			Help: "Total number of login attempts.",
		},
		[]string{"service", "result"},
	)

	TokensIssuedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_tokens_issued_total",
			Help: "Total number of tokens issued or refreshed.",
		},
		[]string{"service", "flow", "result"},
	)
)

func MustRegister(serviceName string) {
	HTTPRequestsTotal = HTTPRequestsTotal.MustCurryWith(prometheus.Labels{"service": serviceName})
	HTTPRequestDurationSeconds = HTTPRequestDurationSeconds.MustCurryWith(prometheus.Labels{"service": serviceName}).(*prometheus.HistogramVec)
	AuthRegistrationsTotal = AuthRegistrationsTotal.MustCurryWith(prometheus.Labels{"service": serviceName})
	AuthLoginsTotal = AuthLoginsTotal.MustCurryWith(prometheus.Labels{"service": serviceName})
	TokensIssuedTotal = TokensIssuedTotal.MustCurryWith(prometheus.Labels{"service": serviceName})

	prometheus.MustRegister(
		HTTPRequestsTotal,
		HTTPRequestDurationSeconds,
		AuthRegistrationsTotal,
		AuthLoginsTotal,
		TokensIssuedTotal,
	)
}
