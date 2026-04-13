package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/nurtidev/medcore/internal/auth/domain"
	"github.com/nurtidev/medcore/internal/auth/service"
)

type contextKey string

const ContextKeyClaims contextKey = "auth_claims"

// JWT extracts and validates the Bearer token, placing Claims into context.
func JWT(svc service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token == "" {
				writeMiddlewareError(w, r, http.StatusUnauthorized, "unauthorized", "missing bearer token")
				return
			}

			claims, err := svc.ValidateToken(r.Context(), token)
			if err != nil {
				switch err {
				case domain.ErrTokenExpired:
					writeMiddlewareError(w, r, http.StatusUnauthorized, "token_expired", "token expired")
				default:
					writeMiddlewareError(w, r, http.StatusUnauthorized, "token_invalid", "token invalid")
				}
				return
			}

			ctx := context.WithValue(r.Context(), ContextKeyClaims, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFromContext retrieves Claims stored by the JWT middleware.
func ClaimsFromContext(ctx context.Context) (*domain.Claims, bool) {
	claims, ok := ctx.Value(ContextKeyClaims).(*domain.Claims)
	return claims, ok
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func writeMiddlewareError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	reqID := r.Header.Get("X-Request-ID")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":      code,
		"message":    message,
		"request_id": reqID,
	})
}
