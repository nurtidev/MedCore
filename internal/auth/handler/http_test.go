package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nurtidev/medcore/internal/auth/domain"
	"github.com/nurtidev/medcore/internal/auth/handler"
	"github.com/nurtidev/medcore/internal/auth/service"
)

// ─── Mock service ─────────────────────────────────────────────────────────────

type mockAuthService struct {
	registerFn      func(ctx context.Context, req domain.RegisterRequest) (*domain.User, error)
	loginFn         func(ctx context.Context, email, password string) (*domain.TokenPair, error)
	refreshFn       func(ctx context.Context, refreshToken string) (*domain.TokenPair, error)
	logoutFn        func(ctx context.Context, refreshToken string) error
	validateTokenFn func(ctx context.Context, accessToken string) (*domain.Claims, error)
	getUserFn       func(ctx context.Context, userID uuid.UUID) (*domain.User, error)
	updateUserFn    func(ctx context.Context, userID uuid.UUID, req domain.UpdateUserRequest) (*domain.User, error)
	changePassFn    func(ctx context.Context, userID uuid.UUID, old, new string) error
	hasPermFn       func(ctx context.Context, userID uuid.UUID, perm domain.Permission) (bool, error)
	listUsersFn     func(ctx context.Context, clinicID uuid.UUID, limit, offset int) ([]*domain.User, int, error)
	deactivateFn    func(ctx context.Context, targetID, callerID uuid.UUID) error
}

func (m *mockAuthService) Register(ctx context.Context, req domain.RegisterRequest) (*domain.User, error) {
	return m.registerFn(ctx, req)
}
func (m *mockAuthService) Login(ctx context.Context, email, password string) (*domain.TokenPair, error) {
	return m.loginFn(ctx, email, password)
}
func (m *mockAuthService) Refresh(ctx context.Context, rt string) (*domain.TokenPair, error) {
	return m.refreshFn(ctx, rt)
}
func (m *mockAuthService) Logout(ctx context.Context, rt string) error {
	return m.logoutFn(ctx, rt)
}
func (m *mockAuthService) ValidateToken(ctx context.Context, at string) (*domain.Claims, error) {
	return m.validateTokenFn(ctx, at)
}
func (m *mockAuthService) GetUser(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return m.getUserFn(ctx, id)
}
func (m *mockAuthService) UpdateUser(ctx context.Context, id uuid.UUID, req domain.UpdateUserRequest) (*domain.User, error) {
	return m.updateUserFn(ctx, id, req)
}
func (m *mockAuthService) ChangePassword(ctx context.Context, id uuid.UUID, o, n string) error {
	return m.changePassFn(ctx, id, o, n)
}
func (m *mockAuthService) HasPermission(ctx context.Context, id uuid.UUID, p domain.Permission) (bool, error) {
	return m.hasPermFn(ctx, id, p)
}
func (m *mockAuthService) ListUsers(ctx context.Context, clinicID uuid.UUID, limit, offset int) ([]*domain.User, int, error) {
	if m.listUsersFn == nil {
		return nil, 0, nil
	}
	return m.listUsersFn(ctx, clinicID, limit, offset)
}
func (m *mockAuthService) DeactivateUser(ctx context.Context, targetID, callerID uuid.UUID) error {
	if m.deactivateFn == nil {
		return nil
	}
	return m.deactivateFn(ctx, targetID, callerID)
}

var _ service.AuthService = (*mockAuthService)(nil)

// ─── Helpers ──────────────────────────────────────────────────────────────────

func buildRouter(svc service.AuthService) http.Handler {
	return handler.NewHTTP(svc, nil, zerolog.Nop())
}

func testUser() *domain.User {
	return &domain.User{
		ID:        uuid.New(),
		ClinicID:  uuid.New(),
		Email:     "test@clinic.kz",
		FirstName: "Test",
		LastName:  "User",
		Role:      domain.RoleDoctor,
		IsActive:  true,
		CreatedAt: time.Now(),
	}
}

func adminClaims() *domain.Claims {
	return &domain.Claims{
		UserID:      uuid.New(),
		ClinicID:    uuid.New(),
		Role:        domain.RoleAdmin,
		Permissions: domain.DefaultRolePermissions[domain.RoleAdmin],
	}
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestRegisterHandler_Success(t *testing.T) {
	svc := &mockAuthService{
		validateTokenFn: func(_ context.Context, _ string) (*domain.Claims, error) {
			return adminClaims(), nil
		},
		registerFn: func(_ context.Context, _ domain.RegisterRequest) (*domain.User, error) {
			return testUser(), nil
		},
	}

	body, _ := json.Marshal(map[string]string{
		"clinic_id":  uuid.New().String(),
		"email":      "new@clinic.kz",
		"password":   "SecurePass123!",
		"first_name": "Айбек",
		"last_name":  "Сатыбалдиев",
		"role":       "doctor",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-admin-token")

	rr := httptest.NewRecorder()
	buildRouter(svc).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestLoginHandler_Returns401_OnBadPassword(t *testing.T) {
	svc := &mockAuthService{
		loginFn: func(_ context.Context, _, _ string) (*domain.TokenPair, error) {
			return nil, domain.ErrInvalidPassword
		},
	}

	body, _ := json.Marshal(map[string]string{
		"email":    "doc@clinic.kz",
		"password": "wrongpass",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	buildRouter(svc).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestMeHandler_Returns401_WithoutToken(t *testing.T) {
	svc := &mockAuthService{
		validateTokenFn: func(_ context.Context, _ string) (*domain.Claims, error) {
			return nil, domain.ErrUnauthorized
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	// No Authorization header

	rr := httptest.NewRecorder()
	buildRouter(svc).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestMeHandler_Returns200_WithValidToken(t *testing.T) {
	user := testUser()
	claims := &domain.Claims{
		UserID:   user.ID,
		ClinicID: user.ClinicID,
		Role:     user.Role,
	}

	svc := &mockAuthService{
		validateTokenFn: func(_ context.Context, _ string) (*domain.Claims, error) {
			return claims, nil
		},
		getUserFn: func(_ context.Context, _ uuid.UUID) (*domain.User, error) {
			return user, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer valid-token")

	rr := httptest.NewRecorder()
	buildRouter(svc).ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, user.Email, resp["email"])
}

func TestRBACMiddleware_Returns403_InsufficientPermissions(t *testing.T) {
	// Doctor tries to access /api/v1/users (admin+ only)
	doctorClaims := &domain.Claims{
		UserID:      uuid.New(),
		ClinicID:    uuid.New(),
		Role:        domain.RoleDoctor,
		Permissions: domain.DefaultRolePermissions[domain.RoleDoctor],
	}

	svc := &mockAuthService{
		validateTokenFn: func(_ context.Context, _ string) (*domain.Claims, error) {
			return doctorClaims, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer doctor-token")

	rr := httptest.NewRecorder()
	buildRouter(svc).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestListUsersHandler_ReturnsUsersForAdminClinic(t *testing.T) {
	claims := adminClaims()
	users := []*domain.User{
		{
			ID:        uuid.New(),
			ClinicID:  claims.ClinicID,
			Email:     "doctor1@clinic.kz",
			FirstName: "Aruzhan",
			LastName:  "S",
			Role:      domain.RoleDoctor,
			IsActive:  true,
			CreatedAt: time.Now(),
		},
		{
			ID:        uuid.New(),
			ClinicID:  claims.ClinicID,
			Email:     "doctor2@clinic.kz",
			FirstName: "Madi",
			LastName:  "T",
			Role:      domain.RoleCoordinator,
			IsActive:  true,
			CreatedAt: time.Now(),
		},
	}

	svc := &mockAuthService{
		validateTokenFn: func(_ context.Context, _ string) (*domain.Claims, error) {
			return claims, nil
		},
		listUsersFn: func(_ context.Context, clinicID uuid.UUID, limit, offset int) ([]*domain.User, int, error) {
			assert.Equal(t, claims.ClinicID, clinicID)
			assert.Equal(t, 50, limit)
			assert.Equal(t, 10, offset)
			return users, len(users), nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/?limit=50&offset=10", nil)
	req.Header.Set("Authorization", "Bearer valid-admin-token")

	rr := httptest.NewRecorder()
	buildRouter(svc).ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Users []map[string]any `json:"users"`
		Total int              `json:"total"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.Len(t, resp.Users, 2)
	assert.Equal(t, 2, resp.Total)
	assert.Equal(t, "doctor1@clinic.kz", resp.Users[0]["email"])
}

func TestDeactivateUserHandler_CallsServiceWithTargetAndCaller(t *testing.T) {
	claims := adminClaims()
	targetID := uuid.New()
	var gotTargetID uuid.UUID
	var gotCallerID uuid.UUID

	svc := &mockAuthService{
		validateTokenFn: func(_ context.Context, _ string) (*domain.Claims, error) {
			return claims, nil
		},
		deactivateFn: func(_ context.Context, targetIDArg, callerIDArg uuid.UUID) error {
			gotTargetID = targetIDArg
			gotCallerID = callerIDArg
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/"+targetID.String(), nil)
	req.Header.Set("Authorization", "Bearer valid-admin-token")

	rr := httptest.NewRecorder()
	buildRouter(svc).ServeHTTP(rr, req)

	require.Equal(t, http.StatusNoContent, rr.Code)
	assert.Equal(t, targetID, gotTargetID)
	assert.Equal(t, claims.UserID, gotCallerID)
}
