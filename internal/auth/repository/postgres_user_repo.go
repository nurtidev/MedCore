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

type postgresUserRepo struct {
	pool *pgxpool.Pool
}

func NewPostgresUserRepo(pool *pgxpool.Pool) UserRepository {
	return &postgresUserRepo{pool: pool}
}

func (r *postgresUserRepo) Create(ctx context.Context, user *domain.User) (*domain.User, error) {
	const q = `
		INSERT INTO users (id, clinic_id, email, password_hash, first_name, last_name, iin, phone, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at, updated_at`

	now := time.Now()
	user.ID = uuid.New()
	user.CreatedAt = now
	user.UpdatedAt = now
	user.IsActive = true

	err := r.pool.QueryRow(ctx, q,
		user.ID, user.ClinicID, user.Email, user.PasswordHash,
		user.FirstName, user.LastName,
		nullableStr(user.IIN), nullableStr(user.Phone),
		string(user.Role), user.IsActive, user.CreatedAt, user.UpdatedAt,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("postgresUserRepo.Create: %w", err)
	}
	return user, nil
}

func (r *postgresUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	const q = `
		SELECT id, clinic_id, email, password_hash, first_name, last_name,
		       COALESCE(iin,''), COALESCE(phone,''), role, is_active, created_at, updated_at
		FROM users WHERE id = $1`

	user, err := r.scanUser(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("postgresUserRepo.GetByID: %w", err)
	}
	return user, nil
}

func (r *postgresUserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	const q = `
		SELECT id, clinic_id, email, password_hash, first_name, last_name,
		       COALESCE(iin,''), COALESCE(phone,''), role, is_active, created_at, updated_at
		FROM users WHERE email = $1`

	user, err := r.scanUser(r.pool.QueryRow(ctx, q, email))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("postgresUserRepo.GetByEmail: %w", err)
	}
	return user, nil
}

func (r *postgresUserRepo) Update(ctx context.Context, user *domain.User) (*domain.User, error) {
	const q = `
		UPDATE users
		SET first_name=$2, last_name=$3, phone=$4, updated_at=$5
		WHERE id=$1
		RETURNING updated_at`

	user.UpdatedAt = time.Now()
	err := r.pool.QueryRow(ctx, q,
		user.ID, user.FirstName, user.LastName,
		nullableStr(user.Phone), user.UpdatedAt,
	).Scan(&user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("postgresUserRepo.Update: %w", err)
	}
	return user, nil
}

func (r *postgresUserRepo) UpdatePasswordHash(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	const q = `UPDATE users SET password_hash=$2, updated_at=NOW() WHERE id=$1`
	_, err := r.pool.Exec(ctx, q, userID, passwordHash)
	if err != nil {
		return fmt.Errorf("postgresUserRepo.UpdatePasswordHash: %w", err)
	}
	return nil
}

func (r *postgresUserRepo) Deactivate(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE users SET is_active=false, updated_at=NOW() WHERE id=$1`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgresUserRepo.Deactivate: %w", err)
	}
	return nil
}

func (r *postgresUserRepo) ListByClinic(ctx context.Context, clinicID uuid.UUID, limit, offset int) ([]*domain.User, int, error) {
	const countQ = `SELECT COUNT(*) FROM users WHERE clinic_id=$1`
	var total int
	if err := r.pool.QueryRow(ctx, countQ, clinicID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("postgresUserRepo.ListByClinic count: %w", err)
	}

	const q = `
		SELECT id, clinic_id, email, password_hash, first_name, last_name,
		       COALESCE(iin,''), COALESCE(phone,''), role, is_active, created_at, updated_at
		FROM users WHERE clinic_id=$1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, q, clinicID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("postgresUserRepo.ListByClinic: %w", err)
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		user, err := r.scanUser(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("postgresUserRepo.ListByClinic scan: %w", err)
		}
		users = append(users, user)
	}
	return users, total, rows.Err()
}

// GetPermissions returns permissions for a role from the in-memory default matrix.
// For dynamic per-clinic permissions, extend this to query role_permissions table.
func (r *postgresUserRepo) GetPermissions(_ context.Context, role domain.Role) ([]domain.Permission, error) {
	perms, ok := domain.DefaultRolePermissions[role]
	if !ok {
		return []domain.Permission{}, nil
	}
	return perms, nil
}

// scanUser scans a single user row from any pgx row scanner.
type rowScanner interface {
	Scan(dest ...any) error
}

func (r *postgresUserRepo) scanUser(row rowScanner) (*domain.User, error) {
	user := &domain.User{}
	var role string
	err := row.Scan(
		&user.ID, &user.ClinicID, &user.Email, &user.PasswordHash,
		&user.FirstName, &user.LastName, &user.IIN, &user.Phone,
		&role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	user.Role = domain.Role(role)
	return user, nil
}

func nullableStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
