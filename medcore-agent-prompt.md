# MedCore — Claude Code Agent Prompt

## 🎯 Цель
Ты — Senior Go Backend Developer. Твоя задача: собрать требования с сайта Astana Hub по 5 модулям платформы Digital Clinic Hub (DCH), спроектировать и разработать единую Go backend платформу **MedCore** — production-ready микросервисную архитектуру.

---

## 📋 ШАГ 1 — Сбор требований с Astana Hub

Используй web fetch/curl для сбора ТЗ по каждому из 5 модулей. Зайди на каждую страницу и извлеки: описание задачи, функциональные требования, ожидаемый эффект, тип продукта.

### URL'ы для парсинга:

```
1. RBAC / Аутентификация:
https://astanahub.com/en/tech_task/tz-na-razrabotku-modulia-autentifikatsii-bezopasnosti-i-rolevoi-modeli-rbac

2. Онлайн-оплаты и биллинг:
https://astanahub.com/en/tech_task/tz-na-razrabotku-modulia-onlain-oplat-i-billinga

3. Интеграция с ГосAPI:
https://astanahub.com/en/tech_task/tz-na-integratsiiu-s-gosudarstvennymi-servisami-i-reestrami-gosapi

4. Интеграция с лабораториями и агрегаторами:
https://astanahub.com/en/tech_task/tz-na-razrabotku-modulia-integratsii-s-vneshnimi-meditsinskimi-agregatorami-i-laboratoriiami

5. BI-аналитика и отчётность по KPI:
https://astanahub.com/en/tech_task/tz-na-razrabotku-podsistemy-bi-analitika-i-otchetnost-po-kpi
```

После парсинга каждой страницы — сохрани требования в `/docs/requirements/` в отдельный markdown файл на каждый модуль.

---

## 📐 ШАГ 2 — Архитектура платформы MedCore

После сбора требований спроектируй следующую структуру:

### Структура репозитория
```
medcore/
├── cmd/
│   ├── auth/           # auth-service entrypoint
│   ├── billing/        # billing-service entrypoint
│   ├── integration/    # integration-service entrypoint
│   ├── analytics/      # analytics-service entrypoint
│   └── gateway/        # API Gateway entrypoint
├── internal/
│   ├── auth/
│   │   ├── domain/     # User, Role, Permission entities
│   │   ├── repository/ # PostgreSQL repo
│   │   ├── service/    # Business logic
│   │   ├── handler/    # gRPC + HTTP handlers
│   │   └── middleware/ # JWT middleware
│   ├── billing/
│   │   ├── domain/     # Invoice, Payment, Subscription entities
│   │   ├── repository/
│   │   ├── service/
│   │   └── handler/
│   ├── integration/
│   │   ├── gosapi/     # ГосAPI клиент (egov.kz)
│   │   ├── laboratory/ # Лаборатории клиент
│   │   ├── aggregator/ # Медагрегаторы клиент
│   │   └── handler/
│   ├── analytics/
│   │   ├── clickhouse/ # ClickHouse repository
│   │   ├── service/    # KPI расчёты
│   │   └── handler/
│   └── shared/
│       ├── config/     # Viper config
│       ├── logger/     # Zerolog
│       ├── database/   # PostgreSQL + ClickHouse connections
│       ├── kafka/      # Producer/Consumer
│       └── errors/     # Domain errors
├── pkg/
│   ├── proto/          # gRPC protobuf definitions
│   └── middleware/     # Shared middleware
├── migrations/         # SQL migrations (goose)
├── docs/
│   ├── requirements/   # Собранные ТЗ с Astana Hub
│   └── architecture/   # ADR документы
├── deployments/
│   ├── docker/
│   └── k8s/
├── docker-compose.yml
├── Makefile
└── README.md
```

### Технологический стек
```
Language:     Go 1.22+
Framework:    net/http + chi router (или fiber)
gRPC:         google.golang.org/grpc
DB:           PostgreSQL 16 (pgx/v5)
Analytics:    ClickHouse (clickhouse-go/v2)
Cache:        Redis 7
Messaging:    Kafka (confluent-kafka-go)
Auth:         JWT (golang-jwt/jwt/v5)
Config:       Viper
Logging:      Zerolog
Tracing:      OpenTelemetry
Metrics:      Prometheus
Migration:    Goose
Testing:      testify + mockery
Container:    Docker + docker-compose
```

---

## 🔐 ШАГ 3 — Разработка модулей

### Модуль 1: auth-service (RBAC)

Реализуй на основе собранных требований с сайта:

```go
// Роли согласно ТЗ DCH
type Role string
const (
    RoleDoctor      Role = "doctor"
    RoleCoordinator Role = "coordinator"
    RoleAdmin       Role = "admin"
    RoleSuperAdmin  Role = "super_admin"
)

// Требования из ТЗ:
// - Жёсткое разграничение прав (врач/координатор/администратор)
// - JWT токены авторизации (access + refresh)
// - Хранение данных согласно законодательству РК о персональных данных
// - Audit log всех действий
```

Реализуй:
- [ ] JWT access token (15 мин) + refresh token (7 дней)
- [ ] RBAC middleware с permission matrix
- [ ] Audit log в PostgreSQL
- [ ] Rate limiting по IP
- [ ] Соответствие ЗРК "О персональных данных"
- [ ] gRPC сервер для межсервисной аутентификации
- [ ] REST API для frontend

### Модуль 2: billing-service (Оплаты + SaaS биллинг)

```go
// Требования из ТЗ:
// - Онлайн-оплата услуг прямо из системы
// - B2B биллинг клиник (SaaS подписки)
// - Управление тарифами и доступами
// - Автоматический контроль оплат
```

Реализуй:
- [ ] Интеграция с Kaspi Pay / CloudPayments (казахстанские эквайеры)
- [ ] Idempotency keys для всех платёжных операций (опыт eMoney.ge)
- [ ] Invoice генерация (PDF)
- [ ] Webhook обработка от платёжных систем
- [ ] SaaS subscription управление (тарифы: Basic/Pro/Enterprise)
- [ ] Автоматическое отключение доступа при неоплате
- [ ] Kafka events: payment.completed, subscription.expired
- [ ] Prometheus метрики: payment_success_total, payment_failed_total

### Модуль 3: integration-service (ГосAPI + Лаборатории)

```go
// Требования из ТЗ:
// - Подключение к внешним медицинским базам (egov.kz)
// - Синхронизация расписания с агрегаторами
// - Автоматическое получение результатов анализов
// - API gateway для внешних сервисов
```

Реализуй:
- [ ] HTTP клиент для egov.kz (ИИН валидация, ЭЦП проверка)
- [ ] Адаптеры для лабораторий (INVITRO, Олимп и др.)
- [ ] Адаптеры для медагрегаторов (Docdoc, 1с-медицина)
- [ ] Circuit breaker (sony/gobreaker)
- [ ] Retry с exponential backoff
- [ ] Кэширование ответов в Redis (TTL по типу данных)
- [ ] Dead letter queue в Kafka для failed integrations
- [ ] Webhook входящие от лабораторий

### Модуль 4: analytics-service (BI + KPI)

```go
// Требования из ТЗ:
// - Визуализация данных по загруженности врачей
// - Финансовая аналитика
// - Эффективность работы клиники
// - Прозрачная аналитика для принятия решений
```

Реализуй:
- [ ] ClickHouse схема для медицинских событий
- [ ] Materialized views для KPI метрик
- [ ] REST API для дашборд данных
- [ ] Агрегации: по врачу, по клинике, по периоду
- [ ] KPI: загруженность врача, revenue per doctor, no-show rate
- [ ] Kafka consumer для сбора событий из всех сервисов
- [ ] Экспорт в Excel/CSV

---

## 🗃️ ШАГ 4 — База данных

### PostgreSQL схема (основная)
```sql
-- auth
CREATE TABLE users (...)
CREATE TABLE roles (...)
CREATE TABLE permissions (...)
CREATE TABLE role_permissions (...)
CREATE TABLE audit_logs (...)
CREATE TABLE refresh_tokens (...)

-- billing
CREATE TABLE invoices (...)
CREATE TABLE payments (...)
CREATE TABLE subscriptions (...)
CREATE TABLE subscription_plans (...)
CREATE TABLE clinics (...)  -- B2B клиенты

-- integration
CREATE TABLE integration_configs (...)
CREATE TABLE sync_logs (...)
CREATE TABLE lab_results (...)
```

### ClickHouse схема (аналитика)
```sql
CREATE TABLE clinic_events (
    event_id UUID,
    clinic_id UUID,
    doctor_id UUID,
    patient_id UUID,
    event_type String,  -- appointment, payment, lab_result
    created_at DateTime,
    metadata String     -- JSON
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(created_at)
ORDER BY (clinic_id, created_at);

-- Materialized views для KPI
CREATE MATERIALIZED VIEW doctor_workload_mv ...
CREATE MATERIALIZED VIEW clinic_revenue_mv ...
```

---

## 🐳 ШАГ 5 — Docker Compose

```yaml
# docker-compose.yml
services:
  postgres:
    image: postgres:16-alpine
    
  clickhouse:
    image: clickhouse/clickhouse-server:24
    
  redis:
    image: redis:7-alpine
    
  kafka:
    image: confluentinc/cp-kafka:7.6.0
    
  zookeeper:
    image: confluentinc/cp-zookeeper:7.6.0

  auth-service:
    build: ./cmd/auth
    
  billing-service:
    build: ./cmd/billing
    
  integration-service:
    build: ./cmd/integration
    
  analytics-service:
    build: ./cmd/analytics
    
  gateway:
    build: ./cmd/gateway
    ports:
      - "8080:8080"
```

---

## 📝 ШАГ 6 — Makefile

```makefile
.PHONY: run build test migrate proto lint

run:
	docker-compose up -d

build:
	go build ./...

test:
	go test -race -coverprofile=coverage.out ./...

migrate-up:
	goose -dir migrations postgres "$(DATABASE_URL)" up

proto:
	protoc --go_out=. --go-grpc_out=. pkg/proto/*.proto

lint:
	golangci-lint run ./...

mock:
	mockery --all --dir internal --output internal/mocks
```

---

## 📄 ШАГ 7 — README.md

Сгенерируй профессиональный README с:
- Описанием платформы MedCore
- Architecture diagram (ASCII)
- Список модулей и их функционал
- Quickstart (docker-compose up)
- API endpoints по каждому сервису
- Ссылкой на задачи Astana Hub DCH
- Стек технологий
- Автор: Nurtilek Assankhan (github.com/nurtidev)

---

## 🚀 ШАГ 8 — Предложение на Astana Hub

После создания структуры проекта и README — сгенерируй текст предложения (300-400 слов) для подачи на задачи DCH через форму на сайте. Текст должен:
- Объяснить что ты закрываешь все 5 модулей единой платформой
- Упомянуть конкретный опыт: eMoney.ge (idempotency, платежи), Sergek Group (ClickHouse аналитика), Darwin Tech Labs (интеграции)
- Указать стек: Go, PostgreSQL, ClickHouse, Kafka, Redis, gRPC
- Сослаться на GitHub репо: github.com/nurtidev/medcore
- Указать готовность к созвону

---

## ⚡ Порядок выполнения

```
1. web_fetch каждого URL → сохранить в docs/requirements/
2. Проанализировать требования → уточнить архитектуру
3. Создать структуру папок и go.mod
4. Написать shared/ пакеты (config, logger, db, kafka)
5. Реализовать auth-service (самый базовый, от него зависят все)
6. Реализовать billing-service
7. Реализовать integration-service
8. Реализовать analytics-service
9. Реализовать API gateway
10. docker-compose.yml + Makefile
11. Написать README.md
12. Сгенерировать текст предложения для Astana Hub
```

---

## 🔑 Важные принципы разработки

- **Idempotency** везде где есть платежи и внешние интеграции
- **Circuit breaker** для всех внешних HTTP вызовов
- **Structured logging** (zerolog) с correlation ID
- **Graceful shutdown** в каждом сервисе
- **Health checks** /health и /ready endpoints
- **OpenTelemetry** трейсинг между сервисами
- **Тесты** минимум на service layer (unit) и handlers (integration)
- **Соответствие законодательству РК** — персональные данные только в зашифрованном виде

---

*Автор: Nurtilek Assankhan | github.com/nurtidev | @nurtilek_assankhan*
*Платформа: MedCore | Заказчик: ТОО DIGITAL CLINIC HUB (Astana Hub)*
