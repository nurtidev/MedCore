package domain

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type Claims struct {
	UserID      uuid.UUID    `json:"uid"`
	ClinicID    uuid.UUID    `json:"cid"`
	Role        Role         `json:"role"`
	Permissions []Permission `json:"perms"`
	jwt.RegisteredClaims
}

type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
	RevokedAt *time.Time
}

type AuditLog struct {
	ID         uuid.UUID
	UserID     *uuid.UUID
	ClinicID   *uuid.UUID
	Action     string
	EntityType string
	EntityID   *uuid.UUID
	IPAddress  string
	UserAgent  string
	Metadata   map[string]any
	CreatedAt  time.Time
}
