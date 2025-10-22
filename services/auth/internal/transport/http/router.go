package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"auth/internal/dto"
	"auth/internal/netutil"
	"auth/internal/service"
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

func NewRouter(auth service.AuthService, tokens service.TokenService) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

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
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(res)
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
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(res)
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
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(res)
	})

	return mux
}
