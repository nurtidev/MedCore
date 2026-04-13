# Prompt 05 — API Gateway

## Контекст проекта

Ты — Senior Go Backend Developer. Разрабатываешь `gateway` для платформы **MedCore** — Go микросервисная медицинская SaaS платформа для казахстанского рынка (заказчик: ТОО Digital Clinic Hub).

**Репозиторий:** `github.com/nurtidev/medcore` (go 1.25, monorepo)

**Что уже реализовано и работает:**
- `internal/shared/` — config (Viper), logger (Zerolog), database (pgx/v5, ClickHouse, Redis), kafka
- `internal/auth/` — JWT RBAC сервис, gRPC ValidateToken/CheckPermission
- `internal/billing/` — Kaspi Pay + Stripe, подписки, инвойсы
- `internal/integration/` — eGov/DAMUMED + iDoctor + Олимп/Инвиво
- `internal/analytics/` — ClickHouse KPI, Kafka consumer
- `pkg/proto/auth/` — **сгенерированные** auth.pb.go + auth_grpc.pb.go (НЕ ручные)

**Все 55 тестов проходят.** `go build ./...` чистый.

---

## Задача

Gateway — единая точка входа для всех клиентских запросов.  
**Не содержит бизнес-логики.** Только:
- Валидация JWT через `auth-service` gRPC
- Reverse proxy к upstream сервисам
- Rate limiting (Redis), CORS, structured logging
- Один агрегирующий endpoint `/api/v1/dashboard`

---

## Структура файлов

```
cmd/gateway/
└── main.go                        # entrypoint, graceful shutdown

internal/gateway/
├── proxy/
│   ├── reverseproxy.go            # базовый httputil.ReverseProxy хелпер
│   ├── auth_proxy.go
│   ├── billing_proxy.go
│   ├── integration_proxy.go
│   └── analytics_proxy.go
├── middleware/
│   ├── auth.go                    # gRPC ValidateToken → X-User-ID headers
│   ├── rate_limit.go              # Redis sliding window
│   ├── cors.go
│   ├── logger.go                  # request logging + X-Request-ID
│   └── tracing.go                 # OpenTelemetry
└── handler/
    ├── health.go                  # /health, /ready
    └── aggregation.go             # GET /api/v1/dashboard (parallel fan-out)

configs/gateway.yaml
deployments/docker/gateway.Dockerfile
```

---

## Маршрутизация (chi router)

```
Порт: 8080 — единственный публичный порт платформы

# Auth Service → :8081
/api/v1/auth/*          → http://auth-service:8081   [без auth]
/api/v1/users/*         → http://auth-service:8081   [auth required]

# Billing Service → :8082
/api/v1/payments/*      → http://billing-service:8082  [auth required]
/api/v1/invoices/*      → http://billing-service:8082  [auth required]
/api/v1/subscriptions/* → http://billing-service:8082  [auth required]
/api/v1/plans/*         → http://billing-service:8082  [без auth — публичные тарифы]
/webhooks/kaspi         → http://billing-service:8082  [без auth — подпись проверяет billing]
/webhooks/stripe        → http://billing-service:8082  [без auth]

# Integration Service → :8083
/api/v1/gov/*           → http://integration-service:8083  [auth required]
/api/v1/sync/*          → http://integration-service:8083  [auth required]
/api/v1/lab-results/*   → http://integration-service:8083  [auth required]
/api/v1/integrations/*  → http://integration-service:8083  [auth required]
/webhooks/idoctor       → http://integration-service:8083  [без auth]
/webhooks/olymp         → http://integration-service:8083  [без auth]
/webhooks/invivo        → http://integration-service:8083  [без auth]

# Analytics Service → :8084
/api/v1/analytics/*     → http://analytics-service:8084  [auth required]

# Gateway own endpoints
GET /health
GET /ready
GET /api/v1/dashboard   # агрегирующий [auth required]
```

---

## Auth Middleware

```go
// internal/gateway/middleware/auth.go

// Whitelist — пропускать без токена:
var authWhitelist = []string{
    "/api/v1/auth/login",
    "/api/v1/auth/register",
    "/api/v1/auth/refresh",
    "/api/v1/plans",
    "/webhooks/",
    "/health",
    "/ready",
}

// Логика:
// 1. Проверить путь в whitelist → пропустить
// 2. Извлечь Bearer токен из Authorization header → 401 если нет
// 3. Вызвать auth-service gRPC: ValidateToken(token) с timeout 5s
// 4. Если valid=false → 401
// 5. Прокинуть в upstream headers:
//    X-User-ID:    {user_id}
//    X-Clinic-ID:  {clinic_id}
//    X-User-Role:  {role}
//    X-Request-ID: {correlation_id}  ← генерируется gateway, не пользователем
// 6. УДАЛИТЬ оригинальный Authorization header из upstream запроса
//    (upstream сервисы доверяют X-User-ID, не перепроверяют JWT)
```

---

## Reverse Proxy

```go
// internal/gateway/proxy/reverseproxy.go

// Использовать стандартный httputil.ReverseProxy
// Для каждого upstream:
//   - timeout: 30s (analytics: 5s — тяжёлые запросы)
//   - снять X-Forwarded-For оригинальный, добавить корректный
//   - при upstream ошибке 5xx → вернуть клиенту {"error": "upstream_error"} + статус
//   - логировать: upstream, duration, status

func NewProxy(target string, timeout time.Duration) http.Handler
```

---

## Rate Limiting

```go
// internal/gateway/middleware/rate_limit.go
// Redis sliding window

// Лимиты:
// Глобально:           1000 req/min per IP
// /api/v1/auth/login:  10 req/min per IP
// /api/v1/analytics/*: 60 req/min per clinic_id (из X-Clinic-ID header)
// При превышении: 429 Too Many Requests + Retry-After header
```

---

## Агрегирующий endpoint

```go
// GET /api/v1/dashboard?clinic_id=&period=
// internal/gateway/handler/aggregation.go

// Параллельно запрашивает (golang/sync errgroup):
//   1. GET http://billing-service:8082/api/v1/subscriptions/current
//   2. GET http://analytics-service:8084/api/v1/analytics/dashboard?clinic_id=&period=
//
// Timeout: 3s на весь запрос
// Partial response: если один сервис не ответил — вернуть то что есть + флаг partial=true
// Прокидывать X-User-ID, X-Clinic-ID в оба upstream запроса

type DashboardResponse struct {
    Subscription *json.RawMessage `json:"subscription,omitempty"`
    Analytics    *json.RawMessage `json:"analytics,omitempty"`
    Partial      bool             `json:"partial,omitempty"`
}
```

---

## Конфигурация

```yaml
# configs/gateway.yaml
server:
  http_port: 8080
  read_timeout: 60s
  write_timeout: 60s

upstream:
  auth:        "http://auth-service:8081"
  billing:     "http://billing-service:8082"
  integration: "http://integration-service:8083"
  analytics:   "http://analytics-service:8084"
  timeouts:
    default:   "30s"
    analytics: "5s"

auth_grpc:
  addr: "auth-service:9091"
  timeout: "5s"

rate_limit:
  global_rpm:    1000
  login_rpm:     10
  analytics_rpm: 60

cors:
  allowed_origins: ["https://app.medcore.kz", "http://localhost:3000", "http://localhost:5173"]
  allowed_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
  allowed_headers: ["Authorization", "Content-Type", "X-Request-ID"]

log:
  level: "info"
  format: "json"
```

---

## gRPC клиент к auth-service

```go
// Переиспользовать типы из pkg/proto/auth (уже сгенерированы):
// authpb "github.com/nurtidev/medcore/pkg/proto/auth"
//
// Создать gRPC клиент при старте:
// conn, err := grpc.Dial(cfg.AuthGRPC.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
// authClient := authpb.NewAuthServiceClient(conn)
//
// В middleware вызывать:
// resp, err := authClient.ValidateToken(ctx, &authpb.ValidateTokenRequest{AccessToken: token})
```

---

## main.go (cmd/gateway/main.go)

```go
// 1. Загрузить configs/gateway.yaml (shared/config)
// 2. Инициализировать zerolog logger
// 3. Подключить Redis (для rate limiting)
// 4. Создать gRPC соединение к auth-service
// 5. Создать chi router со всеми middleware и маршрутами
// 6. Запустить HTTP сервер на :8080
// 7. Graceful shutdown: SIGTERM/SIGINT → HTTP Shutdown(30s) → закрыть gRPC conn → Redis
```

---

## Dockerfile

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o gateway ./cmd/gateway

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/gateway .
COPY --from=builder /app/configs/gateway.yaml ./configs/
EXPOSE 8080
CMD ["./gateway"]
```

---

## Тесты

```
// internal/gateway/middleware/auth_test.go
TestAuthMiddleware_Whitelist_NoToken_Passes
TestAuthMiddleware_MissingToken_Returns401
TestAuthMiddleware_InvalidToken_Returns401
TestAuthMiddleware_ValidToken_SetsHeaders

// internal/gateway/handler/aggregation_test.go
TestDashboard_BothServicesOK
TestDashboard_AnalyticsTimeout_PartialResponse
TestDashboard_BothFail_Returns502
```

---

## Важные принципы

- Gateway **не** обращается в БД — только Redis и gRPC к auth
- `X-Request-ID` генерировать на gateway (uuid.New()), прокидывать во все upstream
- Upstream сервисы **не перепроверяют JWT** — они доверяют `X-User-ID` от gateway
- Удалять `Authorization` header перед проксированием (безопасность)
- При недоступном auth-service gRPC → 503 Service Unavailable (не 401)
- `go build ./...` должен быть чистым после реализации

---

## Что использовать из shared/

```go
// Уже готово — просто импортировать:
"github.com/nurtidev/medcore/internal/shared/config"   // config.Load()
"github.com/nurtidev/medcore/internal/shared/logger"   // logger.New(), logger.HTTPMiddleware()
"github.com/nurtidev/medcore/internal/shared/database" // database.NewRedisClient()
authpb "github.com/nurtidev/medcore/pkg/proto/auth"    // ValidateToken gRPC
```

*Часть платформы MedCore | Автор: Nurtilek Assankhan | github.com/nurtidev*
