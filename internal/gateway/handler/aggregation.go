package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"

	gatewaycfg "github.com/nurtidev/medcore/internal/gateway/config"
)

// DashboardResponse is the aggregated response for GET /api/v1/dashboard.
type DashboardResponse struct {
	Subscription *json.RawMessage `json:"subscription,omitempty"`
	Analytics    *json.RawMessage `json:"analytics,omitempty"`
	Partial      bool             `json:"partial,omitempty"`
}

// Dashboard returns a handler that fans out to billing and analytics in parallel.
// Total timeout is 3 seconds; if one upstream fails the response is partial.
func Dashboard(upstream gatewaycfg.UpstreamConfig) http.HandlerFunc {
	client := &http.Client{Timeout: 4 * time.Second}

	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		clinicID := r.Header.Get("X-Clinic-ID")
		requestID := r.Header.Get("X-Request-ID")

		clinicIDParam := r.URL.Query().Get("clinic_id")
		if clinicIDParam == "" {
			clinicIDParam = clinicID
		}
		period := r.URL.Query().Get("period")

		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		var (
			subscriptionData json.RawMessage
			analyticsData    json.RawMessage
			subErr           error
			anaErr           error
		)

		g, gctx := errgroup.WithContext(ctx)

		g.Go(func() error {
			url := fmt.Sprintf("%s/api/v1/subscriptions/current", upstream.Billing)
			data, err := fetchUpstream(gctx, client, url, userID, clinicIDParam, requestID)
			if err != nil {
				subErr = err
				return nil // don't propagate — allow partial response
			}
			subscriptionData = data
			return nil
		})

		g.Go(func() error {
			url := fmt.Sprintf("%s/api/v1/analytics/dashboard?clinic_id=%s&period=%s",
				upstream.Analytics, clinicIDParam, period)
			data, err := fetchUpstream(gctx, client, url, userID, clinicIDParam, requestID)
			if err != nil {
				anaErr = err
				return nil // don't propagate — allow partial response
			}
			analyticsData = data
			return nil
		})

		_ = g.Wait()

		// Both failed → 502
		if subErr != nil && anaErr != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "upstream_error"})
			return
		}

		resp := DashboardResponse{
			Partial: subErr != nil || anaErr != nil,
		}
		if len(subscriptionData) > 0 {
			resp.Subscription = &subscriptionData
		}
		if len(analyticsData) > 0 {
			resp.Analytics = &analyticsData
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func fetchUpstream(ctx context.Context, client *http.Client, url, userID, clinicID, requestID string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("fetchUpstream: new request: %w", err)
	}
	if userID != "" {
		req.Header.Set("X-User-ID", userID)
	}
	if clinicID != "" {
		req.Header.Set("X-Clinic-ID", clinicID)
	}
	if requestID != "" {
		req.Header.Set("X-Request-ID", requestID)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetchUpstream: do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("fetchUpstream: upstream status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("fetchUpstream: read body: %w", err)
	}

	return json.RawMessage(body), nil
}
