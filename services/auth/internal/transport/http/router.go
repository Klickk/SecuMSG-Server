package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"auth/internal/domain"
	"auth/internal/dto"
	"auth/internal/netutil"
	"auth/internal/service"

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

func NewRouter(auth service.AuthService, devices service.DeviceService, tokens service.TokenService) *http.ServeMux {
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
		var req dto.RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		ip := clientIP(r)
		res, err := auth.Register(r.Context(), req, ip, r.UserAgent())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, res)
	})

	mux.HandleFunc("/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req dto.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		ip := clientIP(r)
		res, err := auth.Login(r.Context(), req, ip, r.UserAgent())
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		writeJSON(w, http.StatusOK, res)
	})

	// Optional: refresh endpoint
	mux.HandleFunc("/v1/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			RefreshToken string `json:"refreshToken"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		ip := clientIP(r)
		res, err := tokens.Refresh(r.Context(), body.RefreshToken, ip, r.UserAgent())
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		writeJSON(w, http.StatusOK, res)
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
		if err := devices.Revoke(r.Context(), domain.DeviceID(deviceID)); err != nil {
			writeDeviceError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
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
	default:
		status = http.StatusBadRequest
	}
	http.Error(w, err.Error(), status)
}
