package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	authpb "github.com/nurtidev/medcore/pkg/proto/auth"
)

// authWhitelist contains path prefixes that are allowed without a JWT token.
var authWhitelist = []string{
	"/api/v1/auth/login",
	"/api/v1/auth/register",
	"/api/v1/auth/refresh",
	"/api/v1/plans",
	"/webhooks/",
	"/health",
	"/ready",
}

type contextKey string

const (
	ctxKeyUserID   contextKey = "user_id"
	ctxKeyClinicID contextKey = "clinic_id"
	ctxKeyRole     contextKey = "role"
)

// Auth validates JWT tokens via the auth-service gRPC and injects user identity headers.
// Paths in authWhitelist are passed through without validation.
func Auth(authClient authpb.AuthServiceClient, grpcTimeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isWhitelisted(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				writeJSON(w, http.StatusUnauthorized, "missing_token")
				return
			}
			token := strings.TrimPrefix(authHeader, "Bearer ")

			ctx, cancel := context.WithTimeout(r.Context(), grpcTimeout)
			defer cancel()

			resp, err := authClient.ValidateToken(ctx, &authpb.ValidateTokenRequest{
				AccessToken: token,
			})
			if err != nil {
				// auth-service unreachable → 503
				writeJSON(w, http.StatusServiceUnavailable, "auth_service_unavailable")
				return
			}
			if !resp.GetValid() {
				writeJSON(w, http.StatusUnauthorized, "invalid_token")
				return
			}

			// Inject identity headers for upstream services.
			r = r.Clone(r.Context())
			r.Header.Set("X-User-ID", resp.GetUserId())
			r.Header.Set("X-Clinic-ID", resp.GetClinicId())
			r.Header.Set("X-User-Role", resp.GetRole())

			// Strip the original Authorization header — upstreams trust X-User-ID.
			r.Header.Del("Authorization")

			next.ServeHTTP(w, r)
		})
	}
}

func isWhitelisted(path string) bool {
	for _, prefix := range authWhitelist {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
