package main

import (
	"context"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"

	"gateway/internal/authz"
	gwmw "gateway/internal/middleware"
	"gateway/internal/proxy"
)

func main() {
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
		log.Fatalf("invalid MESSAGES_BASE_URL: %v", err)
	}
	msgWSProxy := httputil.NewSingleHostReverseProxy(msgWSURL)
	msgWSProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("gateway: websocket proxy error: %v", err)
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}

	r := chi.NewRouter()

	// --- Middlewares ---
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))

	// rate limit (e.g., 100 req / minute by IP)
	r.Use(httprate.LimitByIP(100, 1*time.Minute))

	// CORS
	origins := strings.Split(envOr("CORS_ORIGINS", ""), ",")
	c := cors.Options{
		AllowedOrigins:   originsIfSet(origins),
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Request-Id"},
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

	// -------- Public auth endpoints (pass-through to Auth) --------
	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", p.ForwardJSON("/v1/auth/register"))
		r.Post("/login", p.ForwardJSON("/v1/auth/login"))
		r.Post("/refresh", p.ForwardJSON("/v1/auth/refresh"))
	})

	// -------- Key service proxy --------
	r.Route("/keys", func(r chi.Router) {
		r.Post("/device/register", keysProxy.ForwardJSON("/keys/device/register"))
		r.Get("/bundle", keysProxy.ForwardJSON("/keys/bundle"))
		r.Post("/rotate-signed-prekey", keysProxy.ForwardJSON("/keys/rotate-signed-prekey"))
	})

	// -------- Message service proxy --------
	r.Post("/messages/send", messagesProxy.ForwardJSON("/messages/send"))
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
		log.Println("gateway: using HS256 shared-secret token validation")
		hv := authz.NewHMACValidator(sharedHS, issuer)
		authMW = hv.Middleware
	} else {
		log.Printf("gateway: using JWKS at %s", jwksURL)
		jv, err := authz.NewJWTValidator(context.Background(), jwksURL, issuer)
		if err != nil {
			log.Fatalf("failed to init JWT validator: %v", err)
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

	addr := ":8080"
	log.Printf("gateway listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
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
