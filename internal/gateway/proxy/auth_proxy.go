package proxy

import (
	"net/http"
	"time"
)

// NewAuthProxy creates a reverse proxy to the auth-service.
func NewAuthProxy(target string, timeout time.Duration) http.Handler {
	return NewProxy(target, timeout)
}
