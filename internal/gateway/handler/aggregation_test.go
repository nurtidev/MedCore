package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nurtidev/medcore/internal/gateway/handler"
	gatewaycfg "github.com/nurtidev/medcore/internal/gateway/config"
)

func makeUpstream(billingServer, analyticsServer *httptest.Server) gatewaycfg.UpstreamConfig {
	billing := ""
	analytics := ""
	if billingServer != nil {
		billing = billingServer.URL
	}
	if analyticsServer != nil {
		analytics = analyticsServer.URL
	}
	return gatewaycfg.UpstreamConfig{
		Billing:   billing,
		Analytics: analytics,
		Timeouts: gatewaycfg.TimeoutsConfig{
			Default:   30 * time.Second,
			Analytics: 5 * time.Second,
		},
	}
}

// TestDashboard_BothServicesOK verifies that when both upstreams respond
// successfully the aggregated response contains both payloads.
func TestDashboard_BothServicesOK(t *testing.T) {
	billingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"plan":"pro","active":true}`))
	}))
	defer billingServer.Close()

	analyticsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"visits":42,"revenue":1000}`))
	}))
	defer analyticsServer.Close()

	h := handler.Dashboard(makeUpstream(billingServer, analyticsServer))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard?clinic_id=c1&period=month", nil)
	req.Header.Set("X-User-ID", "u1")
	req.Header.Set("X-Clinic-ID", "c1")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp handler.DashboardResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.NotNil(t, resp.Subscription)
	assert.NotNil(t, resp.Analytics)
	assert.False(t, resp.Partial)
}

// TestDashboard_AnalyticsTimeout_PartialResponse verifies that when the analytics
// upstream times out the response is marked partial but still contains the billing data.
func TestDashboard_AnalyticsTimeout_PartialResponse(t *testing.T) {
	billingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"plan":"basic","active":true}`))
	}))
	defer billingServer.Close()

	// Analytics server sleeps longer than the dashboard 3-second timeout.
	analyticsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer analyticsServer.Close()

	h := handler.Dashboard(makeUpstream(billingServer, analyticsServer))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard?clinic_id=c1", nil)
	req.Header.Set("X-User-ID", "u1")
	req.Header.Set("X-Clinic-ID", "c1")
	w := httptest.NewRecorder()

	// Run with a 4-second test timeout to avoid hanging.
	done := make(chan struct{})
	go func() {
		h.ServeHTTP(w, req)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(4 * time.Second):
		t.Fatal("handler did not return within 4 seconds")
	}

	require.Equal(t, http.StatusOK, w.Code)

	var resp handler.DashboardResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.NotNil(t, resp.Subscription, "billing data should be present")
	assert.Nil(t, resp.Analytics, "analytics data should be absent on timeout")
	assert.True(t, resp.Partial)
}

// TestDashboard_BothFail_Returns502 verifies that when both upstreams fail
// the handler returns 502.
func TestDashboard_BothFail_Returns502(t *testing.T) {
	// Both servers return 500.
	billingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer billingServer.Close()

	analyticsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer analyticsServer.Close()

	h := handler.Dashboard(makeUpstream(billingServer, analyticsServer))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}
