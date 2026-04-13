package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter implements a Redis-backed sliding window rate limiter.
type RateLimiter struct {
	rdb *redis.Client
}

// NewRateLimiter creates a RateLimiter backed by the provided Redis client.
func NewRateLimiter(rdb *redis.Client) *RateLimiter {
	return &RateLimiter{rdb: rdb}
}

// Limit returns middleware that allows at most `limit` requests per `window` per IP.
// On limit exceeded: 429 with Retry-After header.
func (rl *RateLimiter) Limit(key string, limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			bucketKey := fmt.Sprintf("rate:%s:%s", key, ip)

			allowed, retryAfter, err := rl.allow(r.Context(), bucketKey, limit, window)
			if err != nil {
				// Fail open — don't block requests on Redis errors
				next.ServeHTTP(w, r)
				return
			}

			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
				writeMiddlewareError(w, r, http.StatusTooManyRequests, "rate_limited", "too many requests")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// allow uses a Lua script for atomic sliding-window check.
// Returns (allowed, retryAfter, error).
func (rl *RateLimiter) allow(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	now := time.Now()
	windowMs := window.Milliseconds()
	cutoff := now.UnixMilli() - windowMs

	// Atomic sliding window via Lua script
	script := redis.NewScript(`
		local key    = KEYS[1]
		local now    = tonumber(ARGV[1])
		local cutoff = tonumber(ARGV[2])
		local limit  = tonumber(ARGV[3])
		local ttl    = tonumber(ARGV[4])
		local member = ARGV[5]

		redis.call('ZREMRANGEBYSCORE', key, 0, cutoff)
		redis.call('ZADD', key, now, member)
		local count = tonumber(redis.call('ZCARD', key))
		redis.call('PEXPIRE', key, ttl)

		if count <= limit then
			return 1
		end
		return 0
	`)

	member := fmt.Sprintf("%d", now.UnixNano())
	result, err := script.Run(ctx, rl.rdb,
		[]string{key},
		now.UnixMilli(),
		cutoff,
		limit,
		windowMs,
		member,
	).Int()
	if err != nil {
		return true, 0, fmt.Errorf("rate_limit.allow: %w", err)
	}

	if result == 0 {
		return false, window, nil
	}
	return true, 0, nil
}

func clientIP(r *http.Request) string {
	// Trust X-Real-IP / X-Forwarded-For when behind a proxy
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		// First address is the client
		if host, _, err := net.SplitHostPort(ip); err == nil {
			return host
		}
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
