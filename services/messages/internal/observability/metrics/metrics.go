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

	MessagesStoredTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "messages_stored_total",
			Help: "Total number of stored messages.",
		},
		[]string{"service", "chat_type"},
	)

	MessagesCiphertextBytes = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "messages_ciphertext_bytes",
			Help:    "Ciphertext sizes for stored messages.",
			Buckets: prometheus.ExponentialBuckets(64, 2, 10),
		},
		[]string{"service", "chat_type"},
	)

	MessageHistoryFetchedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "messages_history_fetched_total",
			Help: "Total number of history fetch operations.",
		},
		[]string{"service", "scope"},
	)
)

func MustRegister(serviceName string) {
	HTTPRequestsTotal = HTTPRequestsTotal.MustCurryWith(prometheus.Labels{"service": serviceName})
	HTTPRequestDurationSeconds = HTTPRequestDurationSeconds.MustCurryWith(prometheus.Labels{"service": serviceName}).(*prometheus.HistogramVec)
	MessagesStoredTotal = MessagesStoredTotal.MustCurryWith(prometheus.Labels{"service": serviceName})
	MessagesCiphertextBytes = MessagesCiphertextBytes.MustCurryWith(prometheus.Labels{"service": serviceName}).(*prometheus.HistogramVec)
	MessageHistoryFetchedTotal = MessageHistoryFetchedTotal.MustCurryWith(prometheus.Labels{"service": serviceName})

	prometheus.MustRegister(
		HTTPRequestsTotal,
		HTTPRequestDurationSeconds,
		MessagesStoredTotal,
		MessagesCiphertextBytes,
		MessageHistoryFetchedTotal,
	)
}
