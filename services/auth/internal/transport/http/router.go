package transport

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"auth/internal/domain"
	"auth/internal/dto"
	"auth/internal/netutil"
	"auth/internal/observability/metrics"
	"auth/internal/observability/middleware"
	"auth/internal/service"
	"auth/internal/store"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func clientIP(r *http.Request) string {
	// If you put the service behind a proxy later, these will matter.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// XFF can be a list: client, proxy1, proxy2...
		ip := strings.TrimSpace(strings.Split(xff, ",")[0])
		if normalized, ok := netutil.NormalizeIP(ip); ok {
			return normalized
		}
	}
	if xr := r.Header.Get("X-Real-IP"); xr != "" {
		if normalized, ok := netutil.NormalizeIP(xr); ok {
			return normalized
		}
	}
	// Fallback: split host:port
	if normalized, ok := netutil.NormalizeIP(r.RemoteAddr); ok {
		return normalized
	}
	// Last resort: give back whatever we have (may be empty)
	return r.RemoteAddr
}

func NewRouter(auth service.AuthService, devices service.DeviceService, tokens service.TokenService, st *store.Store) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.Handle("/metrics", promhttp.Handler())

	mux.HandleFunc("/v1/auth/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		reqID := middleware.RequestIDFromContext(r.Context())
		traceID := middleware.TraceIDFromContext(r.Context())
		var req dto.RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			metrics.AuthRegistrationsTotal.WithLabelValues("failure").Inc()
			slog.Warn("register decode failed", "error", err, "request_id", reqID, "trace_id", traceID)
			return
		}
		ip := clientIP(r)
		res, err := auth.Register(r.Context(), req, ip, r.UserAgent())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			metrics.AuthRegistrationsTotal.WithLabelValues("failure").Inc()
			slog.Warn("register failed", "error", err, "request_id", reqID, "trace_id", traceID)
			return
		}
		metrics.AuthRegistrationsTotal.WithLabelValues("success").Inc()
		slog.Info("user registered", "user_id", res.UserID, "request_id", reqID, "trace_id", traceID, "has_password", req.Password != "")
		writeJSON(w, http.StatusOK, res)
	})

	mux.HandleFunc("/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		reqID := middleware.RequestIDFromContext(r.Context())
		traceID := middleware.TraceIDFromContext(r.Context())
		var req dto.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			metrics.AuthLoginsTotal.WithLabelValues("failure").Inc()
			slog.Warn("login decode failed", "error", err, "request_id", reqID, "trace_id", traceID)
			return
		}
		ip := clientIP(r)
		res, err := auth.Login(r.Context(), req, ip, r.UserAgent())
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			metrics.AuthLoginsTotal.WithLabelValues("failure").Inc()
			slog.Warn("login failed", "error", err, "request_id", reqID, "trace_id", traceID)
			return
		}
		metrics.AuthLoginsTotal.WithLabelValues("success").Inc()
		slog.Info("login succeeded", "request_id", reqID, "trace_id", traceID)
		writeJSON(w, http.StatusOK, res)
	})

	// Optional: refresh endpoint
	mux.HandleFunc("/v1/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		reqID := middleware.RequestIDFromContext(r.Context())
		traceID := middleware.TraceIDFromContext(r.Context())
		var body struct {
			RefreshToken string `json:"refreshToken"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			metrics.TokensIssuedTotal.WithLabelValues("refresh", "failure").Inc()
			slog.Warn("refresh decode failed", "error", err, "request_id", reqID, "trace_id", traceID)
			return
		}
		ip := clientIP(r)
		res, err := tokens.Refresh(r.Context(), body.RefreshToken, ip, r.UserAgent())
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			metrics.TokensIssuedTotal.WithLabelValues("refresh", "failure").Inc()
			slog.Warn("token refresh failed", "error", err, "request_id", reqID, "trace_id", traceID)
			return
		}
		metrics.TokensIssuedTotal.WithLabelValues("refresh", "success").Inc()
		slog.Info("token refresh succeeded", "request_id", reqID, "trace_id", traceID)
		writeJSON(w, http.StatusOK, res)
	})

	mux.HandleFunc("/v1/auth/verify", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body dto.VerifyRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Token) == "" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		body.Token = strings.TrimSpace(body.Token)
		body.DeviceID = strings.TrimSpace(body.DeviceID)

		res, err := tokens.VerifyAccess(r.Context(), body)
		if err != nil {
			slog.Warn("token verify error", "error", err)
			http.Error(w, "verification failed", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, res)
	})

	mux.HandleFunc("/v1/users/resolve", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if _, ok := requireToken(w, r, tokens, ""); !ok {
			return
		}
		var body dto.ResolveUserRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		user, device, err := devices.ResolveFirstActiveByUsername(r.Context(), body.Username)
		if err != nil {
			if errors.Is(err, domain.ErrRecordNotFound) || errors.Is(err, domain.ErrDeviceNotFound) {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, dto.ResolveUserResponse{
			UserID:   user.ID.String(),
			Username: user.Username,
			DeviceID: device.ID.String(),
		})
	})

	mux.HandleFunc("/v1/users/resolve-device", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if _, ok := requireToken(w, r, tokens, ""); !ok {
			return
		}
		var body dto.ResolveDeviceRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		deviceID, err := uuid.Parse(strings.TrimSpace(body.DeviceID))
		if err != nil {
			http.Error(w, "invalid deviceId", http.StatusBadRequest)
			return
		}
		user, device, err := devices.ResolveActiveByDeviceID(r.Context(), domain.DeviceID(deviceID))
		if err != nil {
			if errors.Is(err, domain.ErrRecordNotFound) || errors.Is(err, domain.ErrDeviceNotFound) {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, dto.ResolveUserResponse{
			UserID:   user.ID.String(),
			Username: user.Username,
			DeviceID: device.ID.String(),
		})
	})

	mux.HandleFunc("/v1/devices/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req dto.DeviceRegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		res, ok := requireToken(w, r, tokens, "")
		if !ok {
			return
		}
		if trimmed := strings.TrimSpace(req.UserID); trimmed != "" && trimmed != res.UserID {
			http.Error(w, "userId does not match token subject", http.StatusForbidden)
			return
		}
		req.UserID = res.UserID
		userID, err := uuid.Parse(strings.TrimSpace(req.UserID))
		if err != nil {
			http.Error(w, "invalid userId", http.StatusBadRequest)
			return
		}
		device, err := devices.Register(r.Context(), domain.UserID(userID), req.Name, req.Platform)
		if err != nil {
			writeDeviceError(w, err)
			return
		}
		resp := struct {
			DeviceID string `json:"deviceId"`
			UserID   string `json:"userId"`
			Name     string `json:"name"`
			Platform string `json:"platform"`
		}{
			DeviceID: device.ID.String(),
			UserID:   device.UserID.String(),
			Name:     device.Name,
			Platform: device.Platform,
		}
		writeJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("/v1/devices/revoke", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body deviceIDRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		deviceID, err := uuid.Parse(strings.TrimSpace(body.DeviceID))
		if err != nil {
			http.Error(w, "invalid deviceId", http.StatusBadRequest)
			return
		}
		if _, ok := requireToken(w, r, tokens, deviceID.String()); !ok {
			return
		}
		if err := devices.Revoke(r.Context(), domain.DeviceID(deviceID)); err != nil {
			writeDeviceError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/v1/users/me", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if st == nil {
			http.Error(w, "store unavailable", http.StatusInternalServerError)
			return
		}
		res, ok := requireToken(w, r, tokens, "")
		if !ok {
			return
		}
		userID, err := uuid.Parse(res.UserID)
		if err != nil {
			http.Error(w, "invalid user id", http.StatusBadRequest)
			return
		}
		deleted, err := st.DeleteUserData(r.Context(), userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		resp := struct {
			Status           string           `json:"status"`
			DeletedResources map[string]int64 `json:"deletedResources"`
			Timestamp        string           `json:"timestamp"`
		}{
			Status:           "deleted",
			DeletedResources: deleted,
			Timestamp:        time.Now().UTC().Format(time.RFC3339Nano),
		}
		writeJSON(w, http.StatusOK, resp)
	})

	return mux
}

type deviceIDRequest struct {
	DeviceID string `json:"deviceId"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}

func writeDeviceError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, domain.ErrDeviceNotFound):
		status = http.StatusNotFound
	case errors.Is(err, domain.ErrDeviceRevoked):
		status = http.StatusConflict
	case errors.Is(err, domain.ErrNoOneTimePrekeys):
		status = http.StatusConflict
	}
	http.Error(w, err.Error(), status)
}

func bearerToken(r *http.Request) string {
	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		return strings.TrimSpace(authz[len("bearer "):])
	}
	if token := strings.TrimSpace(r.URL.Query().Get("access_token")); token != "" {
		return token
	}
	return ""
}

func requireToken(w http.ResponseWriter, r *http.Request, tokens service.TokenService, deviceID string) (*dto.VerifyResponse, bool) {
	token := bearerToken(r)
	if token == "" {
		http.Error(w, "missing bearer token", http.StatusUnauthorized)
		return nil, false
	}
	req := dto.VerifyRequest{Token: token, DeviceID: strings.TrimSpace(deviceID)}
	res, err := tokens.VerifyAccess(r.Context(), req)
	if err != nil {
		slog.Warn("token verification failed", "error", err)
		http.Error(w, "authorization failed", http.StatusUnauthorized)
		return nil, false
	}
	if !res.Valid {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return nil, false
	}
	if req.DeviceID != "" && !res.DeviceAuthorized {
		http.Error(w, "device not authorized for user", http.StatusForbidden)
		return nil, false
	}
	return &res, true
}
