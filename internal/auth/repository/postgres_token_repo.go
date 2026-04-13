package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nurtidev/medcore/internal/auth/domain"
)

type postgresTokenRepo struct {
	pool *pgxpool.Pool
}

func NewPostgresTokenRepo(pool *pgxpool.Pool) TokenRepository {
	return &postgresTokenRepo{pool: pool}
}

func (r *postgresTokenRepo) SaveRefreshToken(ctx context.Context, token *domain.RefreshToken) error {
	const q = `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)`

	_, err := r.pool.Exec(ctx, q,
		token.ID, token.UserID, token.TokenHash, token.ExpiresAt, token.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgresTokenRepo.SaveRefreshToken: %w", err)
	}
	return nil
}

func (r *postgresTokenRepo) GetRefreshTokenByHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	const q = `
		SELECT id, user_id, token_hash, expires_at, created_at, revoked_at
		FROM refresh_tokens WHERE token_hash=$1`

	rt := &domain.RefreshToken{}
	err := r.pool.QueryRow(ctx, q, hash).Scan(
		&rt.ID, &rt.UserID, &rt.TokenHash, &rt.ExpiresAt, &rt.CreatedAt, &rt.RevokedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTokenInvalid
		}
		return nil, fmt.Errorf("postgresTokenRepo.GetRefreshTokenByHash: %w", err)
	}
	return rt, nil
}

func (r *postgresTokenRepo) RevokeRefreshToken(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE refresh_tokens SET revoked_at=$2 WHERE id=$1`
	_, err := r.pool.Exec(ctx, q, id, time.Now())
	if err != nil {
		return fmt.Errorf("postgresTokenRepo.RevokeRefreshToken: %w", err)
	}
	return nil
}

func (r *postgresTokenRepo) CreateAuditLog(ctx context.Context, entry *domain.AuditLog) error {
	const q = `
		INSERT INTO audit_logs (id, user_id, clinic_id, action, entity_type, entity_id, ip_address, user_agent, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	entry.ID = uuid.New()
	entry.CreatedAt = time.Now()

	_, err := r.pool.Exec(ctx, q,
		entry.ID,
		entry.UserID,
		entry.ClinicID,
		entry.Action,
		nullableStr(entry.EntityType),
		entry.EntityID,
		nullableStr(entry.IPAddress),
		nullableStr(entry.UserAgent),
		entry.Metadata,
		entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgresTokenRepo.CreateAuditLog: %w", err)
	}
	return nil
}
