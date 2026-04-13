package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/nurtidev/medcore/internal/auth/domain"
	"github.com/nurtidev/medcore/internal/auth/service"
)

// ─── In-memory stubs ──────────────────────────────────────────────────────────

type stubUserRepo struct {
	users map[string]*domain.User // keyed by email
	byID  map[uuid.UUID]*domain.User
}

func newStubUserRepo() *stubUserRepo {
	return &stubUserRepo{
		users: make(map[string]*domain.User),
		byID:  make(map[uuid.UUID]*domain.User),
	}
}

func (r *stubUserRepo) Create(_ context.Context, u *domain.User) (*domain.User, error) {
	if _, exists := r.users[u.Email]; exists {
		return nil, domain.ErrUserExists
	}
	u.ID = uuid.New()
	u.CreatedAt = time.Now()
	u.UpdatedAt = time.Now()
	u.IsActive = true
	r.users[u.Email] = u
	r.byID[u.ID] = u
	return u, nil
}

func (r *stubUserRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return u, nil
}

func (r *stubUserRepo) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	u, ok := r.users[email]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return u, nil
}

func (r *stubUserRepo) Update(_ context.Context, u *domain.User) (*domain.User, error) {
	r.byID[u.ID] = u
	r.users[u.Email] = u
	return u, nil
}

func (r *stubUserRepo) UpdatePasswordHash(_ context.Context, id uuid.UUID, hash string) error {
	u, ok := r.byID[id]
	if !ok {
		return domain.ErrUserNotFound
	}
	u.PasswordHash = hash
	return nil
}

func (r *stubUserRepo) Deactivate(_ context.Context, id uuid.UUID) error {
	u, ok := r.byID[id]
	if !ok {
		return domain.ErrUserNotFound
	}
	u.IsActive = false
	return nil
}

func (r *stubUserRepo) ListByClinic(_ context.Context, _ uuid.UUID, _, _ int) ([]*domain.User, int, error) {
	return nil, 0, nil
}

func (r *stubUserRepo) GetPermissions(_ context.Context, role domain.Role) ([]domain.Permission, error) {
	return domain.DefaultRolePermissions[role], nil
}

type stubTokenRepo struct {
	tokens map[string]*domain.RefreshToken // keyed by hash
	logs   []*domain.AuditLog
}

func newStubTokenRepo() *stubTokenRepo {
	return &stubTokenRepo{tokens: make(map[string]*domain.RefreshToken)}
}

func (r *stubTokenRepo) SaveRefreshToken(_ context.Context, t *domain.RefreshToken) error {
	r.tokens[t.TokenHash] = t
	return nil
}

func (r *stubTokenRepo) GetRefreshTokenByHash(_ context.Context, hash string) (*domain.RefreshToken, error) {
	t, ok := r.tokens[hash]
	if !ok {
		return nil, domain.ErrTokenInvalid
	}
	return t, nil
}

func (r *stubTokenRepo) RevokeRefreshToken(_ context.Context, id uuid.UUID) error {
	for _, t := range r.tokens {
		if t.ID == id {
			now := time.Now()
			t.RevokedAt = &now
			return nil
		}
	}
	return nil
}

func (r *stubTokenRepo) CreateAuditLog(_ context.Context, entry *domain.AuditLog) error {
	r.logs = append(r.logs, entry)
	return nil
}

// ─── Test helpers ─────────────────────────────────────────────────────────────

func testConfig() service.Config {
	return service.Config{
		JWTSecret:  []byte("test-secret-32-bytes-long-enough!"),
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 7 * 24 * time.Hour,
		IINKey:     []byte("12345678901234567890123456789012"), // 32 bytes
	}
}

func seedUser(t *testing.T, userRepo *stubUserRepo, email string, role domain.Role, active bool) *domain.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), 4) // low cost for tests
	require.NoError(t, err)

	u := &domain.User{
		ID:           uuid.New(),
		ClinicID:     uuid.New(),
		Email:        email,
		PasswordHash: string(hash),
		FirstName:    "Test",
		LastName:     "User",
		Role:         role,
		IsActive:     active,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	userRepo.users[u.Email] = u
	userRepo.byID[u.ID] = u
	return u
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestLogin_Success(t *testing.T) {
	userRepo := newStubUserRepo()
	tokenRepo := newStubTokenRepo()
	svc := service.New(userRepo, tokenRepo, testConfig())

	seedUser(t, userRepo, "doctor@clinic.kz", domain.RoleDoctor, true)

	pair, err := svc.Login(context.Background(), "doctor@clinic.kz", "correct-password")
	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
	assert.True(t, pair.ExpiresAt.After(time.Now()))
}

func TestLogin_WrongPassword(t *testing.T) {
	userRepo := newStubUserRepo()
	tokenRepo := newStubTokenRepo()
	svc := service.New(userRepo, tokenRepo, testConfig())

	seedUser(t, userRepo, "doctor@clinic.kz", domain.RoleDoctor, true)

	_, err := svc.Login(context.Background(), "doctor@clinic.kz", "wrong-password")
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrInvalidPassword))
}

func TestLogin_UserNotFound(t *testing.T) {
	userRepo := newStubUserRepo()
	tokenRepo := newStubTokenRepo()
	svc := service.New(userRepo, tokenRepo, testConfig())

	_, err := svc.Login(context.Background(), "nobody@clinic.kz", "any-password")
	require.Error(t, err)
	// Returns ErrInvalidPassword intentionally (no email enumeration)
	assert.True(t, errors.Is(err, domain.ErrInvalidPassword))
}

func TestLogin_InactiveUser(t *testing.T) {
	userRepo := newStubUserRepo()
	tokenRepo := newStubTokenRepo()
	svc := service.New(userRepo, tokenRepo, testConfig())

	seedUser(t, userRepo, "inactive@clinic.kz", domain.RoleDoctor, false)

	_, err := svc.Login(context.Background(), "inactive@clinic.kz", "correct-password")
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrUserInactive))
}

func TestRefresh_Success(t *testing.T) {
	userRepo := newStubUserRepo()
	tokenRepo := newStubTokenRepo()
	svc := service.New(userRepo, tokenRepo, testConfig())

	seedUser(t, userRepo, "doc@clinic.kz", domain.RoleDoctor, true)

	pair, err := svc.Login(context.Background(), "doc@clinic.kz", "correct-password")
	require.NoError(t, err)

	newPair, err := svc.Refresh(context.Background(), pair.RefreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, newPair.AccessToken)
	assert.NotEqual(t, pair.RefreshToken, newPair.RefreshToken, "refresh token must be rotated")
}

func TestRefresh_Expired(t *testing.T) {
	userRepo := newStubUserRepo()
	tokenRepo := newStubTokenRepo()
	svc := service.New(userRepo, tokenRepo, testConfig())

	seedUser(t, userRepo, "doc@clinic.kz", domain.RoleDoctor, true)

	pair, err := svc.Login(context.Background(), "doc@clinic.kz", "correct-password")
	require.NoError(t, err)

	// Manually expire the token
	for _, rt := range tokenRepo.tokens {
		rt.ExpiresAt = time.Now().Add(-1 * time.Hour)
	}

	_, err = svc.Refresh(context.Background(), pair.RefreshToken)
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrTokenExpired))
}

func TestRefresh_Revoked(t *testing.T) {
	userRepo := newStubUserRepo()
	tokenRepo := newStubTokenRepo()
	svc := service.New(userRepo, tokenRepo, testConfig())

	seedUser(t, userRepo, "doc@clinic.kz", domain.RoleDoctor, true)

	pair, err := svc.Login(context.Background(), "doc@clinic.kz", "correct-password")
	require.NoError(t, err)

	// Manually revoke
	for _, rt := range tokenRepo.tokens {
		now := time.Now()
		rt.RevokedAt = &now
	}

	_, err = svc.Refresh(context.Background(), pair.RefreshToken)
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrTokenRevoked))
}

func TestHasPermission_Doctor_CannotManageBilling(t *testing.T) {
	userRepo := newStubUserRepo()
	tokenRepo := newStubTokenRepo()
	svc := service.New(userRepo, tokenRepo, testConfig())

	doctor := seedUser(t, userRepo, "doctor@clinic.kz", domain.RoleDoctor, true)

	allowed, err := svc.HasPermission(context.Background(), doctor.ID, domain.PermManageBilling)
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestHasPermission_Admin_CanManageUsers(t *testing.T) {
	userRepo := newStubUserRepo()
	tokenRepo := newStubTokenRepo()
	svc := service.New(userRepo, tokenRepo, testConfig())

	admin := seedUser(t, userRepo, "admin@clinic.kz", domain.RoleAdmin, true)

	allowed, err := svc.HasPermission(context.Background(), admin.ID, domain.PermManageUsers)
	require.NoError(t, err)
	assert.True(t, allowed)
}
