package middleware

import (
	"net/http"

	"github.com/nurtidev/medcore/internal/auth/domain"
)

// RequirePermission returns a middleware that enforces at least one of the given permissions.
func RequirePermission(perms ...domain.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				writeMiddlewareError(w, r, http.StatusUnauthorized, "unauthorized", "missing claims")
				return
			}

			if !hasAnyPermission(claims.Permissions, perms) {
				writeMiddlewareError(w, r, http.StatusForbidden, "forbidden", "insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole returns a middleware that enforces at least one of the given roles.
func RequireRole(roles ...domain.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				writeMiddlewareError(w, r, http.StatusUnauthorized, "unauthorized", "missing claims")
				return
			}

			for _, role := range roles {
				if claims.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}

			writeMiddlewareError(w, r, http.StatusForbidden, "forbidden", "insufficient role")
		})
	}
}

func hasAnyPermission(have []domain.Permission, want []domain.Permission) bool {
	set := make(map[domain.Permission]struct{}, len(have))
	for _, p := range have {
		set[p] = struct{}{}
	}
	for _, p := range want {
		if _, ok := set[p]; ok {
			return true
		}
	}
	return false
}
