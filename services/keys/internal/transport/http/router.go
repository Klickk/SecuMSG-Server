package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"keys/internal/dto"
	"keys/internal/observability/metrics"
	"keys/internal/observability/middleware"
	"keys/internal/service"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func NewRouter(svc *service.Service) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.Handle("/metrics", promhttp.Handler())

	mux.HandleFunc("/keys/device/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		reqID := middleware.RequestIDFromContext(r.Context())
		traceID := middleware.TraceIDFromContext(r.Context())
		var req dto.RegisterDeviceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			metrics.DeviceRegistrationsTotal.WithLabelValues("failure").Inc()
			slog.Warn("device registration decode failed", "error", err, "request_id", reqID, "trace_id", traceID)
			return
		}
		res, err := svc.RegisterDevice(r.Context(), req)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, service.ErrInvalidRequest) {
				status = http.StatusBadRequest
			}
			http.Error(w, err.Error(), status)
			metrics.DeviceRegistrationsTotal.WithLabelValues("failure").Inc()
			slog.Warn("device registration failed", "error", err, "request_id", reqID, "trace_id", traceID)
			return
		}
		metrics.DeviceRegistrationsTotal.WithLabelValues("success").Inc()
		slog.Info("device registered", "device_id", res.DeviceID, "user_id", res.UserID, "one_time_prekeys", res.OneTimePreKeys, "request_id", reqID, "trace_id", traceID)
		writeJSON(w, http.StatusCreated, res)
	})

	mux.HandleFunc("/keys/bundle", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		reqID := middleware.RequestIDFromContext(r.Context())
		traceID := middleware.TraceIDFromContext(r.Context())
		deviceIDParam := r.URL.Query().Get("device_id")
		if deviceIDParam == "" {
			http.Error(w, "missing device_id", http.StatusBadRequest)
			metrics.PreKeyBundlesFetchedTotal.WithLabelValues("failure").Inc()
			slog.Warn("prekey bundle missing device id", "request_id", reqID, "trace_id", traceID)
			return
		}
		deviceID, err := uuid.Parse(deviceIDParam)
		if err != nil {
			http.Error(w, "invalid device_id", http.StatusBadRequest)
			metrics.PreKeyBundlesFetchedTotal.WithLabelValues("failure").Inc()
			slog.Warn("prekey bundle invalid device id", "error", err, "request_id", reqID, "trace_id", traceID)
			return
		}
		res, err := svc.GetPreKeyBundle(r.Context(), deviceID)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, service.ErrDeviceNotFound) {
				status = http.StatusNotFound
			}
			http.Error(w, err.Error(), status)
			metrics.PreKeyBundlesFetchedTotal.WithLabelValues("failure").Inc()
			slog.Warn("prekey bundle fetch failed", "error", err, "device_id", deviceID, "request_id", reqID, "trace_id", traceID)
			return
		}
		metrics.PreKeyBundlesFetchedTotal.WithLabelValues("success").Inc()
		slog.Info("prekey bundle fetched", "device_id", res.DeviceID, "has_one_time", res.OneTimePreKey != nil, "request_id", reqID, "trace_id", traceID)
		writeJSON(w, http.StatusOK, res)
	})

	mux.HandleFunc("/keys/rotate-signed-prekey", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		reqID := middleware.RequestIDFromContext(r.Context())
		traceID := middleware.TraceIDFromContext(r.Context())
		var req dto.RotateSignedPreKeyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			metrics.SignedPreKeysRotatedTotal.WithLabelValues("failure").Inc()
			slog.Warn("rotate signed prekey decode failed", "error", err, "request_id", reqID, "trace_id", traceID)
			return
		}
		res, err := svc.RotateSignedPreKey(r.Context(), req)
		if err != nil {
			status := http.StatusInternalServerError
			switch {
			case errors.Is(err, service.ErrInvalidRequest):
				status = http.StatusBadRequest
			case errors.Is(err, service.ErrDeviceNotFound):
				status = http.StatusNotFound
			}
			http.Error(w, err.Error(), status)
			metrics.SignedPreKeysRotatedTotal.WithLabelValues("failure").Inc()
			slog.Warn("rotate signed prekey failed", "error", err, "request_id", reqID, "trace_id", traceID)
			return
		}
		metrics.SignedPreKeysRotatedTotal.WithLabelValues("success").Inc()
		slog.Info("rotated signed prekey", "device_id", res.DeviceID, "added_one_time_keys", res.AddedOneTimeKeys, "request_id", reqID, "trace_id", traceID)
		writeJSON(w, http.StatusOK, res)
	})

	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
