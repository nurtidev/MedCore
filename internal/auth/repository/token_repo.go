package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nurtidev/medcore/internal/auth/domain"
)

//go:generate mockery --name=TokenRepository --outpkg=mocks --output=../mocks

type TokenRepository interface {
	SaveRefreshToken(ctx context.Context, token *domain.RefreshToken) error
	GetRefreshTokenByHash(ctx context.Context, hash string) (*domain.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id uuid.UUID) error
	CreateAuditLog(ctx context.Context, entry *domain.AuditLog) error
}
