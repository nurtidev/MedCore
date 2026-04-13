package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/rs/zerolog/log"
)

// NewProxy creates an httputil.ReverseProxy to the given target with the given timeout.
// On upstream 5xx or connection failure it writes {"error":"upstream_error"} to the client.
func NewProxy(target string, timeout time.Duration) http.Handler {
	targetURL, err := url.Parse(target)
	if err != nil {
		panic(fmt.Sprintf("proxy.NewProxy: invalid target URL %q: %v", target, err))
	}

	transport := &http.Transport{
		ResponseHeaderTimeout: timeout,
	}

	p := httputil.NewSingleHostReverseProxy(targetURL)
	p.Transport = transport

	// Replace director so we strip X-Forwarded-For and re-set it correctly.
	originalDirector := p.Director
	p.Director = func(req *http.Request) {
		originalDirector(req)
		req.Header.Del("X-Forwarded-For")
		if ip := clientIP(req.RemoteAddr); ip != "" {
			req.Header.Set("X-Forwarded-For", ip)
		}
	}

	p.ModifyResponse = func(resp *http.Response) error {
		return nil
	}

	p.ErrorHandler = func(w http.ResponseWriter, r *http.Request, upstreamErr error) {
		log.Ctx(r.Context()).Error().
			Str("upstream", target).
			Err(upstreamErr).
			Msg("upstream error")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "upstream_error"})
	}

	return p
}

// clientIP extracts the IP from addr (strips port).
func clientIP(addr string) string {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}
