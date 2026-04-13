package proxy

import (
	"net/http"
	"time"
)

// NewIntegrationProxy creates a reverse proxy to the integration-service.
func NewIntegrationProxy(target string, timeout time.Duration) http.Handler {
	return NewProxy(target, timeout)
}
