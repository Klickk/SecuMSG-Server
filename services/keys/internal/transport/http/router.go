package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"keys/internal/dto"
	"keys/internal/service"

	"github.com/google/uuid"
)

func NewRouter(svc *service.Service) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/keys/device/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req dto.RegisterDeviceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		res, err := svc.RegisterDevice(r.Context(), req)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, service.ErrInvalidRequest) {
				status = http.StatusBadRequest
			}
			http.Error(w, err.Error(), status)
			return
		}
		writeJSON(w, http.StatusCreated, res)
	})

	mux.HandleFunc("/keys/bundle", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		deviceIDParam := r.URL.Query().Get("device_id")
		if deviceIDParam == "" {
			http.Error(w, "missing device_id", http.StatusBadRequest)
			return
		}
		deviceID, err := uuid.Parse(deviceIDParam)
		if err != nil {
			http.Error(w, "invalid device_id", http.StatusBadRequest)
			return
		}
		res, err := svc.GetPreKeyBundle(r.Context(), deviceID)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, service.ErrDeviceNotFound) {
				status = http.StatusNotFound
			}
			http.Error(w, err.Error(), status)
			return
		}
		writeJSON(w, http.StatusOK, res)
	})

	mux.HandleFunc("/keys/rotate-signed-prekey", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req dto.RotateSignedPreKeyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
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
			return
		}
		writeJSON(w, http.StatusOK, res)
	})

	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
