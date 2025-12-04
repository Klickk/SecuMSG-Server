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

	DeviceRegistrationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "keys_device_registrations_total",
			Help: "Total device registration attempts.",
		},
		[]string{"service", "result"},
	)

	PreKeyBundlesFetchedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "keys_prekey_bundles_fetched_total",
			Help: "Total bundle fetch attempts.",
		},
		[]string{"service", "result"},
	)

	SignedPreKeysRotatedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "keys_signed_prekeys_rotated_total",
			Help: "Total signed prekey rotations.",
		},
		[]string{"service", "result"},
	)
)

func MustRegister(serviceName string) {
	HTTPRequestsTotal = HTTPRequestsTotal.MustCurryWith(prometheus.Labels{"service": serviceName})
	HTTPRequestDurationSeconds = HTTPRequestDurationSeconds.MustCurryWith(prometheus.Labels{"service": serviceName}).(*prometheus.HistogramVec)
	DeviceRegistrationsTotal = DeviceRegistrationsTotal.MustCurryWith(prometheus.Labels{"service": serviceName})
	PreKeyBundlesFetchedTotal = PreKeyBundlesFetchedTotal.MustCurryWith(prometheus.Labels{"service": serviceName})
	SignedPreKeysRotatedTotal = SignedPreKeysRotatedTotal.MustCurryWith(prometheus.Labels{"service": serviceName})

	prometheus.MustRegister(
		HTTPRequestsTotal,
		HTTPRequestDurationSeconds,
		DeviceRegistrationsTotal,
		PreKeyBundlesFetchedTotal,
		SignedPreKeysRotatedTotal,
	)
}
