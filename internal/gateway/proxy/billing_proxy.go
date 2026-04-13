package proxy

import (
	"net/http"
	"time"
)

// NewBillingProxy creates a reverse proxy to the billing-service.
func NewBillingProxy(target string, timeout time.Duration) http.Handler {
	return NewProxy(target, timeout)
}
