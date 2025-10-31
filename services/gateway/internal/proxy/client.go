// services/gateway/internal/proxy/client.go
package proxy

import (
	"bytes"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	hc      *http.Client
	debug   bool
}

func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		hc: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				Proxy:               http.ProxyFromEnvironment,
				MaxIdleConns:        100,
				IdleConnTimeout:     90 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		},
		debug: strings.EqualFold(os.Getenv("GATEWAY_DEBUG"), "true"),
	}
}

// ForwardJSON forwards method/body/headers to auth path, sets real IP headers safely,
// and logs upstream status/duration. It does NOT log request bodies (to avoid password leaks).
func (c *Client) ForwardJSON(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		upURL := c.baseURL + path
		if qs := r.URL.RawQuery; qs != "" {
			upURL += "?" + qs
		}

		// Build upstream request; stream body as-is.
		req, err := http.NewRequestWithContext(r.Context(), r.Method, upURL, r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		// Copy headers (minus hop-by-hop & fragile ones). Ensure JSON if absent.
		req.Header = make(http.Header, len(r.Header))
		for k, vs := range r.Header {
			// Skip hop-by-hop headers per RFC 2616 sec 13.5.1 + Content-Length/Host (managed by net/http)
			switch strings.ToLower(k) {
			case "connection", "keep-alive", "proxy-connection", "transfer-encoding",
				"upgrade", "te", "trailer", "content-length", "host":
				continue
			}
			for _, v := range vs {
				req.Header.Add(k, v)
			}
		}
		if req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}

		// Propagate request id if present
		if rid := r.Header.Get("X-Request-Id"); rid != "" {
			req.Header.Set("X-Request-Id", rid)
		}

		// Real client IP (append to X-Forwarded-For)
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		if host == "" {
			host = r.RemoteAddr
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff == "" {
			req.Header.Set("X-Forwarded-For", host)
		} else {
			req.Header.Set("X-Forwarded-For", xff+", "+host)
		}

		// User-Agent (optional: preserve)
		if ua := r.Header.Get("User-Agent"); ua != "" {
			req.Header.Set("User-Agent", ua)
		}

		resp, err := c.hc.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		// If upstream errored, buffer a small portion for logs, then relay it back.
		var bodyBuf []byte
		var rw io.Reader = resp.Body
		if resp.StatusCode >= 400 && c.debug {
			bodyBuf, _ = io.ReadAll(io.LimitReader(resp.Body, 2048))
			rw = io.MultiReader(bytes.NewReader(bodyBuf), resp.Body)
		}

		// Relay headers and status
		for k, vs := range resp.Header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, rw)

		// Log a concise line for observability
		dur := time.Since(start)
		rid := r.Header.Get("X-Request-Id")
		log.Printf("proxy %-6s %s -> %s %d in %v rid=%s ct=%s",
			r.Method, r.URL.RequestURI(), path, resp.StatusCode, dur, rid, r.Header.Get("Content-Type"))

		// Debug: log a snippet of the upstream error body
		if resp.StatusCode >= 400 && c.debug && len(bodyBuf) > 0 {
			trim := strings.TrimSpace(string(bodyBuf))
			if len(trim) > 500 {
				trim = trim[:500] + "…(truncated)"
			}
			log.Printf("proxy upstream body (status=%d, rid=%s): %q", resp.StatusCode, rid, trim)
		}
	}
}
