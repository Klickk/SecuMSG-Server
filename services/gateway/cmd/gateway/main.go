package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"gateway/internal/authz"
	gwmw "gateway/internal/middleware"
	"gateway/internal/observability/logging"
	"gateway/internal/observability/metrics"
	obsmw "gateway/internal/observability/middleware"
	"gateway/internal/proxy"
)

func main() {
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "dev"
	}

	logger := logging.NewLogger(logging.Config{
		ServiceName: "gateway",
		Environment: env,
		Level:       os.Getenv("LOG_LEVEL"),
	})

	slog.SetDefault(logger)
	metrics.MustRegister("gateway")

	logger.Info("starting service")

	authBase := envOr("AUTH_BASE_URL", "http://localhost:8081")
	keysBase := envOr("KEYS_BASE_URL", "http://localhost:8082")
	messagesBase := envOr("MESSAGES_BASE_URL", "http://localhost:8084")
	issuer := envOr("ISSUER", "http://localhost:8081")
	jwksURL := envOr("JWKS_URL", issuer+"/v1/oauth/jwks")
	sharedHS := os.Getenv("GATEWAY_SHARED_HS256_SECRET") // if set → use HS256 shared secret

	// Prepare Auth proxy client
	p := proxy.New(authBase, 10*time.Second)
	keysProxy := proxy.New(keysBase, 10*time.Second)
	messagesProxy := proxy.New(messagesBase, 10*time.Second)

	msgWSURL, err := url.Parse(messagesBase)
	if err != nil {
		logger.Error("invalid MESSAGES_BASE_URL", "error", err)
		os.Exit(1)
	}
	msgWSProxy := httputil.NewSingleHostReverseProxy(msgWSURL)
	msgWSProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		reqID := obsmw.RequestIDFromContext(r.Context())
		traceID := obsmw.TraceIDFromContext(r.Context())
		slog.Error("gateway websocket proxy error", "error", err, "request_id", reqID, "trace_id", traceID)
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}

	r := chi.NewRouter()

	// --- Middlewares ---
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(TimeoutExceptWS(30 * time.Second))

	// rate limit (e.g., 100 req / minute by IP)
	//r.Use(httprate.LimitByIP(100, 1*time.Minute))

	// CORS
	origins := strings.Split(envOr("CORS_ORIGINS", ""), ",")
	c := cors.Options{
		AllowedOrigins: originsIfSet(origins),
		// Allow any origin (handy for local testing); AllowedOrigins still respected
		// when you want to lock it down via CORS_ORIGINS.
		AllowOriginFunc:  func(r *http.Request, origin string) bool { return true },
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Request-ID", "X-Trace-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}
	r.Use(cors.Handler(c))
	r.Use(chimw.Logger)

	// add X-Request-Id to response for tracing
	r.Use(gwmw.PropagateRequestID())

	// health
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r.Handle("/metrics", promhttp.Handler())

	// -------- Public auth endpoints (pass-through to Auth) --------
	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", p.ForwardJSON("/v1/auth/register"))
		r.Post("/login", p.ForwardJSON("/v1/auth/login"))
		r.Post("/refresh", p.ForwardJSON("/v1/auth/refresh"))
		r.Post("/verify", p.ForwardJSON("/v1/auth/verify"))
		r.Post("/resolve", p.ForwardJSON("/v1/users/resolve"))
		r.Post("/resolve-device", p.ForwardJSON("/v1/users/resolve-device"))
		r.Route("/devices", func(r chi.Router) {
			r.Post("/register", p.ForwardJSON("/v1/devices/register"))
			r.Post("/rotate-prekeys", p.ForwardJSON("/v1/devices/rotate-prekeys"))
			r.Post("/revoke", p.ForwardJSON("/v1/devices/revoke"))
			r.Post("/allocate-prekey", p.ForwardJSON("/v1/devices/allocate-prekey"))
		})
	})

	// -------- Key service proxy --------
	r.Route("/keys", func(r chi.Router) {
		r.Post("/device/register", keysProxy.ForwardJSON("/keys/device/register"))
		r.Get("/bundle", keysProxy.ForwardJSON("/keys/bundle"))
		r.Post("/rotate-signed-prekey", keysProxy.ForwardJSON("/keys/rotate-signed-prekey"))
	})

	// -------- Message service proxy --------
	r.Post("/messages/send", messagesProxy.ForwardJSON("/messages/send"))
	r.Get("/messages/history", messagesProxy.ForwardJSON("/messages/history"))
	r.Get("/messages/conversations", messagesProxy.ForwardJSON("/messages/conversations"))
	wsHandler := func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		msgWSProxy.ServeHTTP(w, req)
	}
	r.HandleFunc("/ws", wsHandler)
	r.HandleFunc("/messages/ws", wsHandler)

	// -------- Protected example route --------
	// choose validator: HS256 shared secret (if provided) else JWKS
	var authMW func(http.Handler) http.Handler
	if sharedHS != "" {
		slog.Info("gateway using HS256 shared-secret token validation")
		hv := authz.NewHMACValidator(sharedHS, issuer)
		authMW = hv.Middleware
	} else {
		slog.Info("gateway using JWKS", "jwks_url", jwksURL)
		jv, err := authz.NewJWTValidator(context.Background(), jwksURL, issuer)
		if err != nil {
			slog.Error("failed to init JWT validator", "error", err)
			os.Exit(1)
		}
		authMW = jv.Middleware
	}

	r.Group(func(pr chi.Router) {
		pr.Use(authMW)

		// This would be a gateway-owned endpoint OR proxy to another service.
		pr.Get("/me", func(w http.ResponseWriter, r *http.Request) {
			sub, ok := authz.SubjectFrom(r.Context())
			if !ok || sub == "" {
				http.Error(w, "no subject", http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"sub":"` + sub + `"}`))
		})
	})

	handler := obsmw.WithRequestAndTrace(obsmw.WithMetrics(r))

	addr := ":8080"
	slog.Info("gateway listening", "addr", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func originsIfSet(in []string) []string {
	out := []string{}
	for _, o := range in {
		if s := strings.TrimSpace(o); s != "" {
			out = append(out, s)
		}
	}
	// Empty slice tells the CORS lib "disallow all" unless you want "*"
	if len(out) == 0 {
		return []string{"*"}
	}
	return out
}

func TimeoutExceptWS(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip if WebSocket upgrade
			if strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") &&
				strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
				next.ServeHTTP(w, r)
				return
			}

			// Standard timeout handling for normal HTTP
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()

			// Use a ResponseWriter that stops writes after context deadline
			done := make(chan struct{})
			tw := &timeoutWriter{ResponseWriter: w}
			go func() {
				next.ServeHTTP(tw, r.WithContext(ctx))
				close(done)
			}()

			select {
			case <-ctx.Done():
				// only write if handler hasn't already started/hijacked
				if !tw.wroteHeader {
					http.Error(w, "timeout", http.StatusGatewayTimeout)
				}
			case <-done:
			}
		})
	}
}

type timeoutWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

func (tw *timeoutWriter) WriteHeader(code int) {
	tw.wroteHeader = true
	tw.ResponseWriter.WriteHeader(code)
}
