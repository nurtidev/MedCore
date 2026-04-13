package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/nurtidev/medcore/internal/gateway/middleware"
	authpb "github.com/nurtidev/medcore/pkg/proto/auth"
)

// mockAuthClient is a test double for authpb.AuthServiceClient.
type mockAuthClient struct {
	resp *authpb.ValidateTokenResponse
	err  error
}

func (m *mockAuthClient) ValidateToken(_ context.Context, _ *authpb.ValidateTokenRequest, _ ...grpc.CallOption) (*authpb.ValidateTokenResponse, error) {
	return m.resp, m.err
}

func (m *mockAuthClient) CheckPermission(_ context.Context, _ *authpb.CheckPermissionRequest, _ ...grpc.CallOption) (*authpb.CheckPermissionResponse, error) {
	return nil, nil
}

// nextHandler records whether it was called and captures identity headers.
type nextHandler struct {
	called   bool
	userID   string
	clinicID string
	role     string
	authHdr  string
}

func (h *nextHandler) ServeHTTP(_ http.ResponseWriter, r *http.Request) {
	h.called = true
	h.userID = r.Header.Get("X-User-ID")
	h.clinicID = r.Header.Get("X-Clinic-ID")
	h.role = r.Header.Get("X-User-Role")
	h.authHdr = r.Header.Get("Authorization")
}

func makeAuthMiddleware(client authpb.AuthServiceClient) func(http.Handler) http.Handler {
	return middleware.Auth(client, 5*time.Second)
}

// TestAuthMiddleware_Whitelist_NoToken_Passes verifies that whitelisted paths
// pass through without a token.
func TestAuthMiddleware_Whitelist_NoToken_Passes(t *testing.T) {
	next := &nextHandler{}
	mw := makeAuthMiddleware(&mockAuthClient{}) // client never called

	handler := mw(next)

	whitelistedPaths := []string{
		"/api/v1/auth/login",
		"/api/v1/auth/register",
		"/api/v1/auth/refresh",
		"/api/v1/plans",
		"/webhooks/kaspi",
		"/health",
		"/ready",
	}

	for _, path := range whitelistedPaths {
		t.Run(path, func(t *testing.T) {
			next.called = false
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.True(t, next.called, "handler should be called for whitelisted path %s", path)
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

// TestAuthMiddleware_MissingToken_Returns401 verifies that non-whitelisted paths
// without a Bearer token get a 401.
func TestAuthMiddleware_MissingToken_Returns401(t *testing.T) {
	next := &nextHandler{}
	mw := makeAuthMiddleware(&mockAuthClient{})
	handler := mw(next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, next.called)
}

// TestAuthMiddleware_InvalidToken_Returns401 verifies that a token that auth-service
// marks invalid results in 401.
func TestAuthMiddleware_InvalidToken_Returns401(t *testing.T) {
	next := &nextHandler{}
	client := &mockAuthClient{
		resp: &authpb.ValidateTokenResponse{Valid: false},
	}
	mw := makeAuthMiddleware(client)
	handler := mw(next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, next.called)
}

// TestAuthMiddleware_ValidToken_SetsHeaders verifies that a valid token causes
// X-User-ID / X-Clinic-ID / X-User-Role to be forwarded and Authorization
// to be stripped.
func TestAuthMiddleware_ValidToken_SetsHeaders(t *testing.T) {
	next := &nextHandler{}
	client := &mockAuthClient{
		resp: &authpb.ValidateTokenResponse{
			Valid:    true,
			UserId:   "user-42",
			ClinicId: "clinic-7",
			Role:     "doctor",
		},
	}
	mw := makeAuthMiddleware(client)
	h := mw(next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.True(t, next.called)
	assert.Equal(t, "user-42", next.userID)
	assert.Equal(t, "clinic-7", next.clinicID)
	assert.Equal(t, "doctor", next.role)
	assert.Empty(t, next.authHdr, "Authorization header must be stripped before upstream")
}
