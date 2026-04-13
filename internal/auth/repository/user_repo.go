package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nurtidev/medcore/internal/auth/domain"
)

//go:generate mockery --name=UserRepository --outpkg=mocks --output=../mocks

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) (*domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) (*domain.User, error)
	UpdatePasswordHash(ctx context.Context, userID uuid.UUID, passwordHash string) error
	Deactivate(ctx context.Context, id uuid.UUID) error
	ListByClinic(ctx context.Context, clinicID uuid.UUID, limit, offset int) ([]*domain.User, int, error)
	GetPermissions(ctx context.Context, role domain.Role) ([]domain.Permission, error)
}
