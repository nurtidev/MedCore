# Prompt 01 — auth-service (RBAC + JWT)

## Источник требований
- ТЗ: `docs/requirements/01-auth-rbac.md`
- Заказчик: ТОО DIGITAL CLINIC HUB
- Дедлайн: 15.04.2026

## Контекст

Ты — Senior Go Backend Developer. Разрабатываешь `auth-service` для платформы **MedCore**.

Это **фундаментальный сервис** — от него зависят billing, integration, analytics. Пишется первым. Должен быть полностью покрыт тестами и готов к production.

Из реального ТЗ заказчика: *"базовый, но очень важный модуль для защиты персональных медицинских данных"* с жёстким разграничением прав доступа и соответствием ЗРК о персональных данных.

---

## Структура файлов

```
cmd/auth/
└── main.go

internal/auth/
├── domain/
│   ├── user.go          # User, Role, Permission entities
│   ├── token.go         # TokenPair, Claims
│   └── errors.go        # sentinel errors
├── repository/
│   ├── user_repo.go         # interface
│   ├── postgres_user_repo.go
│   ├── token_repo.go        # interface
│   └── postgres_token_repo.go
├── service/
│   ├── auth_service.go
│   └── auth_service_test.go
├── handler/
│   ├── http.go
│   ├── http_test.go
│   └── grpc.go
└── middleware/
    ├── jwt.go
    ├── rbac.go
    └── rate_limit.go

pkg/proto/auth/auth.proto
migrations/
├── 001_create_users.sql
├── 002_create_roles_permissions.sql
├── 003_create_refresh_tokens.sql
└── 004_create_audit_logs.sql
configs/auth.yaml
deployments/docker/auth.Dockerfile
```

---

## Domain

```go
// internal/auth/domain/user.go

type Role string

const (
    RoleDoctor      Role = "doctor"       // из ТЗ DCH
    RoleCoordinator Role = "coordinator"  // из ТЗ DCH
    RoleAdmin       Role = "admin"        // из ТЗ DCH
    RoleSuperAdmin  Role = "super_admin"  // SaaS уровень
)

type Permission string

const (
    PermViewPatients  Permission = "patients:read"
    PermEditPatients  Permission = "patients:write"
    PermViewSchedule  Permission = "schedule:read"
    PermEditSchedule  Permission = "schedule:write"
    PermViewBilling   Permission = "billing:read"
    PermManageBilling Permission = "billing:manage"
    PermViewAnalytics Permission = "analytics:read"
    PermManageUsers   Permission = "users:manage"
    PermManageClinics Permission = "clinics:manage"
)

// Permission matrix — из ТЗ DCH
var DefaultRolePermissions = map[Role][]Permission{
    RoleDoctor:      {PermViewPatients, PermViewSchedule},
    RoleCoordinator: {PermViewPatients, PermEditPatients, PermViewSchedule, PermEditSchedule, PermViewBilling},
    RoleAdmin:       {PermViewPatients, PermEditPatients, PermViewSchedule, PermEditSchedule, PermViewBilling, PermManageBilling, PermViewAnalytics, PermManageUsers},
    RoleSuperAdmin:  { /* все permissions */ },
}

type User struct {
    ID           uuid.UUID
    ClinicID     uuid.UUID  // B2B — привязка к клинике
    Email        string
    PasswordHash string
    FirstName    string
    LastName     string
    IIN          string    // зашифрован AES-256-GCM (требование ЗРК о персональных данных)
    Phone        string
    Role         Role
    IsActive     bool
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

---

## Migrations (Goose)

### 001_create_users.sql
```sql
-- +goose Up
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    clinic_id     UUID NOT NULL,
    email         VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    first_name    VARCHAR(100) NOT NULL,
    last_name     VARCHAR(100) NOT NULL,
    iin           VARCHAR(512),       -- AES-256-GCM зашифрован
    phone         VARCHAR(20),
    role          VARCHAR(50) NOT NULL DEFAULT 'doctor',
    is_active     BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_clinic_id ON users(clinic_id);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_role ON users(role);

-- +goose Down
DROP TABLE users;
```

### 002_create_roles_permissions.sql
```sql
-- +goose Up
CREATE TABLE roles (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(50) UNIQUE NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE permissions (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(100) UNIQUE NOT NULL,
    description TEXT
);

CREATE TABLE role_permissions (
    role_id       UUID REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

-- +goose Down
DROP TABLE role_permissions;
DROP TABLE permissions;
DROP TABLE roles;
```

### 003_create_refresh_tokens.sql
```sql
-- +goose Up
CREATE TABLE refresh_tokens (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_hash ON refresh_tokens(token_hash);

-- +goose Down
DROP TABLE refresh_tokens;
```

### 004_create_audit_logs.sql
```sql
-- +goose Up
CREATE TABLE audit_logs (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID REFERENCES users(id) ON DELETE SET NULL,
    clinic_id   UUID,
    action      VARCHAR(100) NOT NULL,
    entity_type VARCHAR(100),
    entity_id   UUID,
    ip_address  INET,
    user_agent  TEXT,
    metadata    JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_clinic_id ON audit_logs(clinic_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);

-- +goose Down
DROP TABLE audit_logs;
```

---

## Service Interface

```go
type AuthService interface {
    Register(ctx context.Context, req RegisterRequest) (*User, error)
    Login(ctx context.Context, email, password string) (*TokenPair, error)
    Refresh(ctx context.Context, refreshToken string) (*TokenPair, error)
    Logout(ctx context.Context, refreshToken string) error
    ValidateToken(ctx context.Context, accessToken string) (*Claims, error)
    GetUser(ctx context.Context, userID uuid.UUID) (*User, error)
    UpdateUser(ctx context.Context, userID uuid.UUID, req UpdateUserRequest) (*User, error)
    ChangePassword(ctx context.Context, userID uuid.UUID, old, new string) error
    HasPermission(ctx context.Context, userID uuid.UUID, perm Permission) (bool, error)
}

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
```

**Бизнес-правила:**
- Access token TTL: **15 минут**
- Refresh token TTL: **7 дней**, хранить только hash (bcrypt) в БД
- Пароли: bcrypt cost=12
- ИИН: шифровать AES-256-GCM, ключ из env `IIN_ENCRYPTION_KEY`
- Permissions грузить при login и включать в Claims — не делать запрос на каждый API вызов
- При logout: `revoked_at = NOW()` для refresh token
- При login неактивного пользователя: вернуть `ErrUserInactive`

---

## REST API (chi router)

```
POST   /api/v1/auth/register        # только admin/super_admin
POST   /api/v1/auth/login
POST   /api/v1/auth/refresh
POST   /api/v1/auth/logout          # [auth required]
GET    /api/v1/auth/me              # [auth required]
PUT    /api/v1/auth/me              # [auth required]
POST   /api/v1/auth/change-password # [auth required]

GET    /api/v1/users                # [admin+]
GET    /api/v1/users/{id}           # [admin+]
PUT    /api/v1/users/{id}           # [admin+]
DELETE /api/v1/users/{id}           # деактивация is_active=false [admin+]

GET    /health
GET    /ready
```

**Формат ошибок:**
```json
{"error": "unauthorized", "message": "token expired", "request_id": "uuid"}
```

---

## gRPC (для межсервисной авторизации)

```protobuf
// pkg/proto/auth/auth.proto
syntax = "proto3";
package auth;
option go_package = "medcore/pkg/proto/auth";

service AuthService {
    rpc ValidateToken(ValidateTokenRequest) returns (ValidateTokenResponse);
    rpc CheckPermission(CheckPermissionRequest) returns (CheckPermissionResponse);
}

message ValidateTokenRequest  { string access_token = 1; }
message ValidateTokenResponse {
    bool   valid       = 1;
    string user_id     = 2;
    string clinic_id   = 3;
    string role        = 4;
    repeated string permissions = 5;
}
message CheckPermissionRequest  { string user_id = 1; string permission = 2; }
message CheckPermissionResponse { bool allowed = 1; }
```

---

## Middleware

```go
// jwt.go — Bearer токен из Authorization header → Claims в context
// rbac.go — RequirePermission(perms ...Permission) и RequireRole(roles ...Role)
// rate_limit.go — Redis sliding window:
//   /auth/login    → 10 req/min per IP
//   /auth/register → 5 req/min per IP
//   Ответ: 429 с Retry-After header
```

---

## Audit Log Events

| action | триггер |
|---|---|
| `user.login` | успешный вход |
| `user.login_failed` | неверный пароль / не найден |
| `user.logout` | выход |
| `user.register` | новый пользователь |
| `user.update` | изменение профиля |
| `user.password_change` | смена пароля |
| `user.deactivate` | деактивация |
| `token.refresh` | обновление токена |

---

## Конфигурация

```yaml
# configs/auth.yaml
server:
  http_port: 8081
  grpc_port: 9091
  read_timeout: 30s
  write_timeout: 30s

database:
  dsn: "${DATABASE_URL}"
  max_open_conns: 25
  max_idle_conns: 5

redis:
  addr: "${REDIS_ADDR}"

jwt:
  secret: "${JWT_SECRET}"
  access_ttl: "15m"
  refresh_ttl: "168h"

encryption:
  iin_key: "${IIN_ENCRYPTION_KEY}"  # 32 байта hex

log:
  level: "info"
  format: "json"
```

---

## Тесты

```
// service/auth_service_test.go
TestLogin_Success
TestLogin_WrongPassword
TestLogin_UserNotFound
TestLogin_InactiveUser
TestRefresh_Success
TestRefresh_Expired
TestRefresh_Revoked
TestHasPermission_Doctor_CannotManageBilling
TestHasPermission_Admin_CanManageUsers

// handler/http_test.go
TestRegisterHandler_Success
TestLoginHandler_Returns401_OnBadPassword
TestMeHandler_Returns401_WithoutToken
TestMeHandler_Returns200_WithValidToken
TestRBACMiddleware_Returns403_InsufficientPermissions
```

---

## Dockerfile

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o auth-service ./cmd/auth

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/auth-service .
COPY --from=builder /app/configs/auth.yaml ./configs/
EXPOSE 8081 9091
CMD ["./auth-service"]
```

---

## Принципы

- Никаких ORM — только `pgx/v5` с `pgxpool.Pool`
- Все секреты из env, никогда не хардкодить
- Ошибки оборачивать: `fmt.Errorf("auth.Login: %w", err)`
- `correlation_id` в context и в каждом лог-записи
- Zerolog: `log.Info().Str("user_id", ...).Str("action", ...).Msg(...)`
- Graceful shutdown: HTTP (30s) → gRPC → pgxpool → Redis
- Никаких паник в production коде
