package proxy

import (
	"net/http"
	"time"
)

// NewAnalyticsProxy creates a reverse proxy to the analytics-service.
// Uses a shorter timeout since analytics queries can be heavy.
func NewAnalyticsProxy(target string, timeout time.Duration) http.Handler {
	return NewProxy(target, timeout)
}
