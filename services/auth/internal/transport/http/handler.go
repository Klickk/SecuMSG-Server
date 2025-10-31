package http

import (
	"net/http"
)

// You can replace this with chi/echo later and wire real handlers.
func NewHTTPHandler(_ interface{}) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	// TODO: mount sub-handlers:
	// mux.Handle("/v1/auth/", registerHandler(...))
	// mux.Handle("/v1/devices/", deviceHandler(...))
	return mux
}
