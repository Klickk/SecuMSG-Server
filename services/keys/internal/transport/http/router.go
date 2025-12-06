package transport

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"keys/internal/auth"
	"keys/internal/dto"
	"keys/internal/observability/metrics"
	"keys/internal/observability/middleware"
	"keys/internal/service"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func extractToken(r *http.Request) string {
	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		return strings.TrimSpace(authz[len("bearer "):])
	}
	if token := strings.TrimSpace(r.URL.Query().Get("access_token")); token != "" {
		return token
	}
	return ""
}

func NewRouter(svc *service.Service, authClient *auth.Client) *http.ServeMux {
	mux := http.NewServeMux()

	requireAuth := func(w http.ResponseWriter, r *http.Request, deviceID uuid.UUID) (auth.Claims, bool) {
		if authClient == nil {
			http.Error(w, "authorization not configured", http.StatusInternalServerError)
			return auth.Claims{}, false
		}
		token := extractToken(r)
		if token == "" {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return auth.Claims{}, false
		}
		claims, err := authClient.Verify(r.Context(), token, deviceID)
		if err != nil {
			slog.Warn("auth verification failed", "error", err)
			http.Error(w, "authorization failed", http.StatusUnauthorized)
			return auth.Claims{}, false
		}
		if !claims.Valid {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return auth.Claims{}, false
		}
		if deviceID != uuid.Nil && !claims.DeviceAuthorized {
			http.Error(w, "device not authorized for user", http.StatusForbidden)
			return auth.Claims{}, false
		}
		return claims, true
	}

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
		deviceUUID := uuid.Nil
		if trimmed := strings.TrimSpace(req.DeviceID); trimmed != "" {
			var err error
			deviceUUID, err = uuid.Parse(trimmed)
			if err != nil {
				http.Error(w, "invalid deviceId", http.StatusBadRequest)
				metrics.DeviceRegistrationsTotal.WithLabelValues("failure").Inc()
				return
			}
			req.DeviceID = deviceUUID.String()
		} else {
			http.Error(w, "deviceId is required", http.StatusBadRequest)
			metrics.DeviceRegistrationsTotal.WithLabelValues("failure").Inc()
			return
		}
		claims, ok := requireAuth(w, r, deviceUUID)
		if !ok {
			return
		}
		req.UserID = claims.UserID.String()
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
		if _, ok := requireAuth(w, r, uuid.Nil); !ok {
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
		devID, err := uuid.Parse(strings.TrimSpace(req.DeviceID))
		if err != nil {
			http.Error(w, "invalid deviceId", http.StatusBadRequest)
			metrics.SignedPreKeysRotatedTotal.WithLabelValues("failure").Inc()
			return
		}
		if _, ok := requireAuth(w, r, devID); !ok {
			return
		}
		req.DeviceID = devID.String()
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
