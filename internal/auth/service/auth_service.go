package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/nurtidev/medcore/internal/auth/domain"
	"github.com/nurtidev/medcore/internal/auth/repository"
	"github.com/nurtidev/medcore/internal/shared/logger"
)

const bcryptCost = 12

// AuthService defines all auth operations.
type AuthService interface {
	Register(ctx context.Context, req domain.RegisterRequest) (*domain.User, error)
	Login(ctx context.Context, email, password string) (*domain.TokenPair, error)
	Refresh(ctx context.Context, refreshToken string) (*domain.TokenPair, error)
	Logout(ctx context.Context, refreshToken string) error
	ValidateToken(ctx context.Context, accessToken string) (*domain.Claims, error)
	GetUser(ctx context.Context, userID uuid.UUID) (*domain.User, error)
	UpdateUser(ctx context.Context, userID uuid.UUID, req domain.UpdateUserRequest) (*domain.User, error)
	ChangePassword(ctx context.Context, userID uuid.UUID, oldPass, newPass string) error
	HasPermission(ctx context.Context, userID uuid.UUID, perm domain.Permission) (bool, error)
	ListUsers(ctx context.Context, clinicID uuid.UUID, limit, offset int) ([]*domain.User, int, error)
	DeactivateUser(ctx context.Context, targetID uuid.UUID, callerID uuid.UUID) error
}

// Config holds service-level configuration.
type Config struct {
	JWTSecret  []byte
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	IINKey     []byte // 32 bytes for AES-256-GCM
}

type authService struct {
	userRepo  repository.UserRepository
	tokenRepo repository.TokenRepository
	cfg       Config
}

// New creates a new AuthService instance.
func New(userRepo repository.UserRepository, tokenRepo repository.TokenRepository, cfg Config) AuthService {
	return &authService{
		userRepo:  userRepo,
		tokenRepo: tokenRepo,
		cfg:       cfg,
	}
}

func (s *authService) Register(ctx context.Context, req domain.RegisterRequest) (*domain.User, error) {
	if req.Email == "" || req.Password == "" || req.FirstName == "" || req.LastName == "" {
		return nil, fmt.Errorf("auth.Register: %w", domain.ErrInvalidInput)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("auth.Register: hash password: %w", err)
	}

	encIIN := ""
	if req.IIN != "" {
		encIIN, err = s.encryptIIN(req.IIN)
		if err != nil {
			return nil, fmt.Errorf("auth.Register: encrypt IIN: %w", err)
		}
	}

	user := &domain.User{
		ClinicID:     req.ClinicID,
		Email:        req.Email,
		PasswordHash: string(hash),
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		IIN:          encIIN,
		Phone:        req.Phone,
		Role:         req.Role,
	}

	created, err := s.userRepo.Create(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("auth.Register: %w", err)
	}

	s.auditLog(ctx, &domain.AuditLog{
		UserID:     &created.ID,
		ClinicID:   &created.ClinicID,
		Action:     "user.register",
		EntityType: "user",
		EntityID:   &created.ID,
	})

	l := logger.FromContext(ctx)
	l.Info().
		Str("user_id", created.ID.String()).
		Str("action", "user.register").
		Msg("user registered")

	return created, nil
}

func (s *authService) Login(ctx context.Context, email, password string) (*domain.TokenPair, error) {
	log := logger.FromContext(ctx)

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		s.auditLog(ctx, &domain.AuditLog{
			Action:   "user.login_failed",
			Metadata: map[string]any{"email": email, "reason": "user not found"},
		})
		log.Warn().Str("email", email).Str("action", "user.login_failed").Msg("user not found")
		// Return generic error to prevent email enumeration
		return nil, fmt.Errorf("auth.Login: %w", domain.ErrInvalidPassword)
	}

	if !user.IsActive {
		s.auditLog(ctx, &domain.AuditLog{
			UserID:   &user.ID,
			ClinicID: &user.ClinicID,
			Action:   "user.login_failed",
			Metadata: map[string]any{"reason": "user inactive"},
		})
		return nil, fmt.Errorf("auth.Login: %w", domain.ErrUserInactive)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		s.auditLog(ctx, &domain.AuditLog{
			UserID:   &user.ID,
			ClinicID: &user.ClinicID,
			Action:   "user.login_failed",
			Metadata: map[string]any{"reason": "invalid password"},
		})
		log.Warn().Str("user_id", user.ID.String()).Str("action", "user.login_failed").Msg("invalid password")
		return nil, fmt.Errorf("auth.Login: %w", domain.ErrInvalidPassword)
	}

	perms, err := s.userRepo.GetPermissions(ctx, user.Role)
	if err != nil {
		return nil, fmt.Errorf("auth.Login: get permissions: %w", err)
	}

	pair, err := s.generateTokenPair(ctx, user, perms)
	if err != nil {
		return nil, fmt.Errorf("auth.Login: %w", err)
	}

	s.auditLog(ctx, &domain.AuditLog{
		UserID:   &user.ID,
		ClinicID: &user.ClinicID,
		Action:   "user.login",
	})
	log.Info().Str("user_id", user.ID.String()).Str("action", "user.login").Msg("user logged in")

	return pair, nil
}

func (s *authService) Refresh(ctx context.Context, refreshToken string) (*domain.TokenPair, error) {
	hash := hashToken(refreshToken)

	rt, err := s.tokenRepo.GetRefreshTokenByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("auth.Refresh: %w", err)
	}

	if rt.RevokedAt != nil {
		return nil, fmt.Errorf("auth.Refresh: %w", domain.ErrTokenRevoked)
	}
	if time.Now().After(rt.ExpiresAt) {
		return nil, fmt.Errorf("auth.Refresh: %w", domain.ErrTokenExpired)
	}

	// Rotate: revoke old, issue new
	if err := s.tokenRepo.RevokeRefreshToken(ctx, rt.ID); err != nil {
		return nil, fmt.Errorf("auth.Refresh: revoke old token: %w", err)
	}

	user, err := s.userRepo.GetByID(ctx, rt.UserID)
	if err != nil {
		return nil, fmt.Errorf("auth.Refresh: get user: %w", err)
	}
	if !user.IsActive {
		return nil, fmt.Errorf("auth.Refresh: %w", domain.ErrUserInactive)
	}

	perms, err := s.userRepo.GetPermissions(ctx, user.Role)
	if err != nil {
		return nil, fmt.Errorf("auth.Refresh: get permissions: %w", err)
	}

	pair, err := s.generateTokenPair(ctx, user, perms)
	if err != nil {
		return nil, fmt.Errorf("auth.Refresh: %w", err)
	}

	s.auditLog(ctx, &domain.AuditLog{
		UserID:   &user.ID,
		ClinicID: &user.ClinicID,
		Action:   "token.refresh",
	})

	return pair, nil
}

func (s *authService) Logout(ctx context.Context, refreshToken string) error {
	hash := hashToken(refreshToken)

	rt, err := s.tokenRepo.GetRefreshTokenByHash(ctx, hash)
	if err != nil {
		return fmt.Errorf("auth.Logout: %w", err)
	}

	if err := s.tokenRepo.RevokeRefreshToken(ctx, rt.ID); err != nil {
		return fmt.Errorf("auth.Logout: %w", err)
	}

	s.auditLog(ctx, &domain.AuditLog{
		UserID: &rt.UserID,
		Action: "user.logout",
	})

	return nil
}

func (s *authService) ValidateToken(_ context.Context, accessToken string) (*domain.Claims, error) {
	claims := &domain.Claims{}
	token, err := jwt.ParseWithClaims(accessToken, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, domain.ErrTokenInvalid
		}
		return s.cfg.JWTSecret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, domain.ErrTokenExpired
		}
		return nil, domain.ErrTokenInvalid
	}
	if !token.Valid {
		return nil, domain.ErrTokenInvalid
	}
	return claims, nil
}

func (s *authService) GetUser(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("auth.GetUser: %w", err)
	}
	return user, nil
}

func (s *authService) UpdateUser(ctx context.Context, userID uuid.UUID, req domain.UpdateUserRequest) (*domain.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("auth.UpdateUser: %w", err)
	}

	if req.FirstName != nil {
		user.FirstName = *req.FirstName
	}
	if req.LastName != nil {
		user.LastName = *req.LastName
	}
	if req.Phone != nil {
		user.Phone = *req.Phone
	}

	updated, err := s.userRepo.Update(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("auth.UpdateUser: %w", err)
	}

	s.auditLog(ctx, &domain.AuditLog{
		UserID:     &userID,
		ClinicID:   &user.ClinicID,
		Action:     "user.update",
		EntityType: "user",
		EntityID:   &userID,
	})

	return updated, nil
}

func (s *authService) ChangePassword(ctx context.Context, userID uuid.UUID, oldPass, newPass string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("auth.ChangePassword: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPass)); err != nil {
		return fmt.Errorf("auth.ChangePassword: %w", domain.ErrInvalidPassword)
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPass), bcryptCost)
	if err != nil {
		return fmt.Errorf("auth.ChangePassword: hash: %w", err)
	}

	if err := s.userRepo.UpdatePasswordHash(ctx, userID, string(newHash)); err != nil {
		return fmt.Errorf("auth.ChangePassword: update: %w", err)
	}

	s.auditLog(ctx, &domain.AuditLog{
		UserID:   &userID,
		ClinicID: &user.ClinicID,
		Action:   "user.password_change",
	})

	return nil
}

func (s *authService) ListUsers(ctx context.Context, clinicID uuid.UUID, limit, offset int) ([]*domain.User, int, error) {
	users, total, err := s.userRepo.ListByClinic(ctx, clinicID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("auth.ListUsers: %w", err)
	}
	return users, total, nil
}

func (s *authService) DeactivateUser(ctx context.Context, targetID uuid.UUID, callerID uuid.UUID) error {
	target, err := s.userRepo.GetByID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("auth.DeactivateUser: %w", err)
	}

	if err := s.userRepo.Deactivate(ctx, targetID); err != nil {
		return fmt.Errorf("auth.DeactivateUser: %w", err)
	}

	s.auditLog(ctx, &domain.AuditLog{
		UserID:     &callerID,
		ClinicID:   &target.ClinicID,
		Action:     "user.deactivate",
		EntityType: "user",
		EntityID:   &targetID,
	})

	return nil
}

func (s *authService) HasPermission(ctx context.Context, userID uuid.UUID, perm domain.Permission) (bool, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("auth.HasPermission: %w", err)
	}
	perms, err := s.userRepo.GetPermissions(ctx, user.Role)
	if err != nil {
		return false, fmt.Errorf("auth.HasPermission: %w", err)
	}
	for _, p := range perms {
		if p == perm {
			return true, nil
		}
	}
	return false, nil
}

// generateTokenPair creates a new access + refresh token pair and persists the refresh token.
func (s *authService) generateTokenPair(ctx context.Context, user *domain.User, perms []domain.Permission) (*domain.TokenPair, error) {
	now := time.Now()
	accessExpiry := now.Add(s.cfg.AccessTTL)

	claims := domain.Claims{
		UserID:      user.ID,
		ClinicID:    user.ClinicID,
		Role:        user.Role,
		Permissions: perms,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(accessExpiry),
		},
	}

	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.cfg.JWTSecret)
	if err != nil {
		return nil, fmt.Errorf("generateTokenPair: sign access token: %w", err)
	}

	// Generate cryptographically random refresh token
	rawRefresh := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, rawRefresh); err != nil {
		return nil, fmt.Errorf("generateTokenPair: generate refresh token: %w", err)
	}
	refreshTokenStr := hex.EncodeToString(rawRefresh)

	rt := &domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: hashToken(refreshTokenStr),
		ExpiresAt: now.Add(s.cfg.RefreshTTL),
		CreatedAt: now,
	}
	if err := s.tokenRepo.SaveRefreshToken(ctx, rt); err != nil {
		return nil, fmt.Errorf("generateTokenPair: save refresh token: %w", err)
	}

	return &domain.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshTokenStr,
		ExpiresAt:    accessExpiry,
	}, nil
}

// encryptIIN encrypts the plaintext IIN using AES-256-GCM.
func (s *authService) encryptIIN(plaintext string) (string, error) {
	block, err := aes.NewCipher(s.cfg.IINKey)
	if err != nil {
		return "", fmt.Errorf("encryptIIN: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("encryptIIN: new gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("encryptIIN: nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// hashToken returns the SHA-256 hex digest of the token (for DB storage and lookup).
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// auditLog fires-and-forgets an audit entry; errors are swallowed intentionally.
func (s *authService) auditLog(ctx context.Context, entry *domain.AuditLog) {
	_ = s.tokenRepo.CreateAuditLog(ctx, entry)
}
