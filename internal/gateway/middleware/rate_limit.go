package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimitConfig holds per-endpoint rate limits (requests per minute).
type RateLimitConfig struct {
	GlobalRPM    int
	LoginRPM     int
	AnalyticsRPM int
}

// RateLimit returns a middleware that enforces sliding-window rate limits via Redis.
//
// Rules applied in order:
//  1. /api/v1/auth/login  → LoginRPM per IP
//  2. /api/v1/analytics/* → AnalyticsRPM per X-Clinic-ID
//  3. everything else     → GlobalRPM per IP
func RateLimit(rdb *redis.Client, cfg RateLimitConfig) func(http.Handler) http.Handler {
	rl := &rateLimiter{rdb: rdb}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			var allowed bool
			var err error

			switch {
			case path == "/api/v1/auth/login":
				ip := extractIP(r)
				allowed, err = rl.check(r.Context(), fmt.Sprintf("rl:login:%s", ip), cfg.LoginRPM)

			case strings.HasPrefix(path, "/api/v1/analytics"):
				clinicID := r.Header.Get("X-Clinic-ID")
				if clinicID == "" {
					clinicID = extractIP(r)
				}
				allowed, err = rl.check(r.Context(), fmt.Sprintf("rl:analytics:%s", clinicID), cfg.AnalyticsRPM)

			default:
				ip := extractIP(r)
				allowed, err = rl.check(r.Context(), fmt.Sprintf("rl:global:%s", ip), cfg.GlobalRPM)
			}

			if err != nil {
				// Fail open — don't block requests on Redis errors.
				next.ServeHTTP(w, r)
				return
			}

			if !allowed {
				w.Header().Set("Retry-After", "60")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "rate_limit_exceeded"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

type rateLimiter struct {
	rdb *redis.Client
}

// check implements a Redis sliding window counter (1-minute window).
// Returns true if the request is within the allowed limit.
func (rl *rateLimiter) check(ctx context.Context, key string, limit int) (bool, error) {
	now := time.Now()
	windowStart := now.Add(-time.Minute)

	// Atomic sliding window via pipeline (acceptable precision for our limits).
	pipe := rl.rdb.TxPipeline()
	pipe.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%d", windowStart.UnixMilli()))
	countCmd := pipe.ZCard(ctx, key)
	member := fmt.Sprintf("%d-%d", now.UnixNano(), now.UnixNano()%1000)
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(now.UnixMilli()), Member: member})
	pipe.Expire(ctx, key, 2*time.Minute)

	if _, err := pipe.Exec(ctx); err != nil {
		return true, fmt.Errorf("rate_limit: redis: %w", err)
	}

	// countCmd was evaluated before our ZAdd, so it reflects count before this request.
	return countCmd.Val() < int64(limit), nil
}

// extractIP returns the client IP, preferring X-Forwarded-For.
func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	return clientAddrIP(r.RemoteAddr)
}

func clientAddrIP(addr string) string {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}
