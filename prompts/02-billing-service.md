# Prompt 02 — billing-service (Онлайн-оплаты + SaaS биллинг)

## Источник требований
- ТЗ: `docs/requirements/02-billing.md`
- Заказчик: ТОО DIGITAL CLINIC HUB

## Контекст

Разрабатываешь `billing-service` для платформы **MedCore**.

Из реального ТЗ заказчика: *"Внедрение возможности оплачивать услуги прямо из системы"*. Указанные интеграции: **Kaspi Pay** и **Stripe**. Роли: пациент (платит), бухгалтер (сверка), администратор (управление).

Дополнительно к требованиям ТЗ — реализуем B2B SaaS биллинг клиник (тарифные планы, автоотключение при неоплате).

**Зависимость:** сервис использует `auth-service` через gRPC для валидации токенов.

---

## Структура файлов

```
cmd/billing/
└── main.go

internal/billing/
├── domain/
│   ├── invoice.go        # Invoice, InvoiceStatus
│   ├── payment.go        # Payment, PaymentStatus, PaymentProvider
│   ├── subscription.go   # Subscription, Plan, PlanTier
│   └── errors.go
├── repository/
│   ├── invoice_repo.go
│   ├── postgres_invoice_repo.go
│   ├── payment_repo.go
│   ├── postgres_payment_repo.go
│   ├── subscription_repo.go
│   └── postgres_subscription_repo.go
├── service/
│   ├── billing_service.go
│   ├── billing_service_test.go
│   ├── invoice_service.go
│   └── subscription_service.go
├── handler/
│   ├── http.go
│   ├── http_test.go
│   ├── webhook.go        # обработка вебхуков от Kaspi Pay / Stripe
│   └── grpc.go
└── provider/
    ├── provider.go       # interface PaymentProvider
    ├── kaspi/
    │   └── kaspi_pay.go  # Kaspi Pay клиент
    └── stripe/
        └── stripe.go     # Stripe клиент

migrations/
├── 005_create_subscription_plans.sql
├── 006_create_subscriptions.sql
├── 007_create_invoices.sql
└── 008_create_payments.sql

configs/billing.yaml
deployments/docker/billing.Dockerfile
```

---

## Domain

```go
// internal/billing/domain/payment.go

type PaymentProvider string
const (
    ProviderKaspi  PaymentProvider = "kaspi"
    ProviderStripe PaymentProvider = "stripe"
)

type PaymentStatus string
const (
    PaymentStatusPending    PaymentStatus = "pending"
    PaymentStatusProcessing PaymentStatus = "processing"
    PaymentStatusCompleted  PaymentStatus = "completed"
    PaymentStatusFailed     PaymentStatus = "failed"
    PaymentStatusRefunded   PaymentStatus = "refunded"
)

type Payment struct {
    ID             uuid.UUID
    InvoiceID      uuid.UUID
    ClinicID       uuid.UUID
    PatientID      uuid.UUID
    IdempotencyKey string         // уникальный ключ — защита от дублей
    Provider       PaymentProvider
    ExternalID     string         // ID транзакции на стороне провайдера
    Amount         decimal.Decimal
    Currency       string         // "KZT", "USD"
    Status         PaymentStatus
    FailureReason  string
    Metadata       map[string]any
    CreatedAt      time.Time
    UpdatedAt      time.Time
}

// internal/billing/domain/invoice.go

type InvoiceStatus string
const (
    InvoiceStatusDraft   InvoiceStatus = "draft"
    InvoiceStatusSent    InvoiceStatus = "sent"
    InvoiceStatusPaid    InvoiceStatus = "paid"
    InvoiceStatusOverdue InvoiceStatus = "overdue"
    InvoiceStatusVoided  InvoiceStatus = "voided"
)

type Invoice struct {
    ID          uuid.UUID
    ClinicID    uuid.UUID
    PatientID   uuid.UUID
    ServiceName string
    Amount      decimal.Decimal
    Currency    string
    Status      InvoiceStatus
    DueAt       time.Time
    PaidAt      *time.Time
    PDFUrl      string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

// internal/billing/domain/subscription.go

type PlanTier string
const (
    PlanBasic      PlanTier = "basic"
    PlanPro        PlanTier = "pro"
    PlanEnterprise PlanTier = "enterprise"
)

type SubscriptionStatus string
const (
    SubStatusActive    SubscriptionStatus = "active"
    SubStatusPastDue   SubscriptionStatus = "past_due"
    SubStatusCancelled SubscriptionStatus = "cancelled"
    SubStatusExpired   SubscriptionStatus = "expired"
)

type Plan struct {
    ID             uuid.UUID
    Tier           PlanTier
    Name           string
    PriceMonthly   decimal.Decimal
    Currency       string
    MaxDoctors     int
    MaxPatients    int
    Features       []string
}

type Subscription struct {
    ID         uuid.UUID
    ClinicID   uuid.UUID
    PlanID     uuid.UUID
    Status     SubscriptionStatus
    CurrentPeriodStart time.Time
    CurrentPeriodEnd   time.Time
    CancelledAt        *time.Time
}
```

---

## Migrations (Goose)

### 005_create_subscription_plans.sql
```sql
-- +goose Up
CREATE TABLE subscription_plans (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tier          VARCHAR(50) NOT NULL,
    name          VARCHAR(100) NOT NULL,
    price_monthly NUMERIC(10,2) NOT NULL,
    currency      VARCHAR(3) NOT NULL DEFAULT 'KZT',
    max_doctors   INT NOT NULL DEFAULT 5,
    max_patients  INT NOT NULL DEFAULT 500,
    features      JSONB,
    is_active     BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed данные
INSERT INTO subscription_plans (tier, name, price_monthly, max_doctors, max_patients, features) VALUES
('basic',      'Basic',      49900,  3,   300,  '["online_payments","basic_analytics"]'),
('pro',        'Pro',        99900,  10,  1000, '["online_payments","advanced_analytics","lab_integrations"]'),
('enterprise', 'Enterprise', 199900, 999, 9999, '["all_features","dedicated_support","custom_integrations"]');

-- +goose Down
DROP TABLE subscription_plans;
```

### 006_create_subscriptions.sql
```sql
-- +goose Up
CREATE TABLE subscriptions (
    id                   UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    clinic_id            UUID NOT NULL,
    plan_id              UUID NOT NULL REFERENCES subscription_plans(id),
    status               VARCHAR(50) NOT NULL DEFAULT 'active',
    current_period_start TIMESTAMPTZ NOT NULL,
    current_period_end   TIMESTAMPTZ NOT NULL,
    cancelled_at         TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_subscriptions_clinic_id ON subscriptions(clinic_id);
CREATE INDEX idx_subscriptions_status ON subscriptions(status);
CREATE INDEX idx_subscriptions_period_end ON subscriptions(current_period_end);

-- +goose Down
DROP TABLE subscriptions;
```

### 007_create_invoices.sql
```sql
-- +goose Up
CREATE TABLE invoices (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    clinic_id    UUID NOT NULL,
    patient_id   UUID,
    service_name VARCHAR(255) NOT NULL,
    amount       NUMERIC(10,2) NOT NULL,
    currency     VARCHAR(3) NOT NULL DEFAULT 'KZT',
    status       VARCHAR(50) NOT NULL DEFAULT 'draft',
    due_at       TIMESTAMPTZ,
    paid_at      TIMESTAMPTZ,
    pdf_url      VARCHAR(500),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_invoices_clinic_id ON invoices(clinic_id);
CREATE INDEX idx_invoices_status ON invoices(status);
CREATE INDEX idx_invoices_patient_id ON invoices(patient_id);

-- +goose Down
DROP TABLE invoices;
```

### 008_create_payments.sql
```sql
-- +goose Up
CREATE TABLE payments (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    invoice_id      UUID NOT NULL REFERENCES invoices(id),
    clinic_id       UUID NOT NULL,
    patient_id      UUID,
    idempotency_key VARCHAR(255) UNIQUE NOT NULL,  -- защита от дублей
    provider        VARCHAR(50) NOT NULL,
    external_id     VARCHAR(255),
    amount          NUMERIC(10,2) NOT NULL,
    currency        VARCHAR(3) NOT NULL DEFAULT 'KZT',
    status          VARCHAR(50) NOT NULL DEFAULT 'pending',
    failure_reason  TEXT,
    metadata        JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_payments_idempotency ON payments(idempotency_key);
CREATE INDEX idx_payments_invoice_id ON payments(invoice_id);
CREATE INDEX idx_payments_clinic_id ON payments(clinic_id);
CREATE INDEX idx_payments_status ON payments(status);

-- +goose Down
DROP TABLE payments;
```

---

## Service Interface

```go
type BillingService interface {
    // Платежи
    CreatePaymentLink(ctx context.Context, req CreatePaymentRequest) (*PaymentLink, error)
    GetPayment(ctx context.Context, paymentID uuid.UUID) (*Payment, error)
    ProcessWebhook(ctx context.Context, provider PaymentProvider, payload []byte, signature string) error

    // Инвойсы
    CreateInvoice(ctx context.Context, req CreateInvoiceRequest) (*Invoice, error)
    GetInvoice(ctx context.Context, invoiceID uuid.UUID) (*Invoice, error)
    ListInvoices(ctx context.Context, clinicID uuid.UUID, filter InvoiceFilter) ([]*Invoice, error)
    GenerateInvoicePDF(ctx context.Context, invoiceID uuid.UUID) ([]byte, error)

    // Подписки
    GetSubscription(ctx context.Context, clinicID uuid.UUID) (*Subscription, error)
    CreateSubscription(ctx context.Context, clinicID uuid.UUID, planID uuid.UUID) (*Subscription, error)
    CancelSubscription(ctx context.Context, clinicID uuid.UUID) error
    CheckSubscriptionAccess(ctx context.Context, clinicID uuid.UUID) (bool, error) // активна ли подписка
}

// Payment Provider interface (Kaspi Pay + Stripe реализуют его)
type PaymentProvider interface {
    CreatePaymentLink(ctx context.Context, req PaymentLinkRequest) (string, error)
    VerifyWebhookSignature(payload []byte, signature string) bool
    ParseWebhookEvent(payload []byte) (*WebhookEvent, error)
    RefundPayment(ctx context.Context, externalID string, amount decimal.Decimal) error
}
```

**Бизнес-правила:**
- `idempotency_key` обязателен для всех `CreatePayment` запросов — если ключ уже есть, вернуть существующий платёж
- После успешного webhook от провайдера: обновить статус payment → completed, invoice → paid
- При `subscription.expired` (CRON каждые 5 минут): отправить Kafka event `subscription.expired`
- Kafka event `payment.completed` при каждом успешном платеже — analytics-service слушает

---

## REST API

```
POST   /api/v1/payments                       # создать ссылку на оплату (пациент/admin)
GET    /api/v1/payments/{id}                  # статус платежа
POST   /api/v1/webhooks/kaspi                 # webhook от Kaspi Pay
POST   /api/v1/webhooks/stripe                # webhook от Stripe

POST   /api/v1/invoices                       # создать счёт [admin]
GET    /api/v1/invoices                       # список счётов [admin/бухгалтер]
GET    /api/v1/invoices/{id}                  # получить счёт
GET    /api/v1/invoices/{id}/pdf              # скачать PDF

GET    /api/v1/subscriptions/current          # текущая подписка клиники [admin]
POST   /api/v1/subscriptions                  # оформить подписку [admin]
DELETE /api/v1/subscriptions/current          # отменить подписку [admin]
GET    /api/v1/plans                          # список тарифных планов

GET    /health
GET    /ready
```

---

## Webhook обработка

```go
// internal/billing/handler/webhook.go

// Kaspi Pay webhook:
// 1. Проверить HMAC-SHA256 подпись (X-Kaspi-Signature header)
// 2. Найти payment по external_id
// 3. Обновить статус атомарно (BEGIN/COMMIT)
// 4. Отправить Kafka event payment.completed
// 5. Вернуть 200 OK немедленно (Kaspi ждёт быстрого ответа)

// Stripe webhook:
// 1. Проверить подпись через stripe.ConstructEvent
// 2. Обработать events: payment_intent.succeeded, payment_intent.failed
// 3. Идемпотентно — повторные webhooks должны обрабатываться без ошибок
```

---

## Kafka Events

```go
// Топики и структуры событий

// payment.completed
type PaymentCompletedEvent struct {
    PaymentID  string          `json:"payment_id"`
    InvoiceID  string          `json:"invoice_id"`
    ClinicID   string          `json:"clinic_id"`
    PatientID  string          `json:"patient_id"`
    Amount     decimal.Decimal `json:"amount"`
    Currency   string          `json:"currency"`
    Provider   string          `json:"provider"`
    OccurredAt time.Time       `json:"occurred_at"`
}

// subscription.expired
type SubscriptionExpiredEvent struct {
    SubscriptionID string    `json:"subscription_id"`
    ClinicID       string    `json:"clinic_id"`
    ExpiredAt      time.Time `json:"expired_at"`
}
```

---

## Prometheus метрики

```go
payment_requests_total{provider, status}    // счётчик
payment_amount_total{provider, currency}    // сумма
payment_duration_seconds{provider}          // гистограмма
subscription_active_total                   // gauge
```

---

## CRON задачи

```go
// Каждые 5 минут: проверить подписки у которых current_period_end < NOW()
// → статус past_due → отправить subscription.expired в Kafka
// Каждый день в 00:00: пометить инвойсы с due_at < NOW() как overdue
```

---

## Конфигурация

```yaml
# configs/billing.yaml
server:
  http_port: 8082
  grpc_port: 9092

database:
  dsn: "${DATABASE_URL}"

kafka:
  brokers: "${KAFKA_BROKERS}"
  topics:
    payment_completed: "payment.completed"
    subscription_expired: "subscription.expired"

kaspi:
  api_url: "${KASPI_API_URL}"
  merchant_id: "${KASPI_MERCHANT_ID}"
  secret_key: "${KASPI_SECRET_KEY}"

stripe:
  secret_key: "${STRIPE_SECRET_KEY}"
  webhook_secret: "${STRIPE_WEBHOOK_SECRET}"

auth_grpc:
  addr: "auth-service:9091"

log:
  level: "info"
  format: "json"
```

---

## Тесты

```
// service/billing_service_test.go
TestCreatePaymentLink_Success
TestCreatePaymentLink_IdempotencyKey_ReturnsSamePayment
TestProcessWebhook_Kaspi_Success
TestProcessWebhook_InvalidSignature_Returns401
TestCreateSubscription_Success
TestCheckSubscriptionAccess_Expired_ReturnsFalse

// handler/http_test.go
TestPaymentWebhookHandler_Kaspi
TestPaymentWebhookHandler_Stripe
TestInvoiceListHandler_FilterByStatus
TestGenerateInvoicePDF
```

---

## Dockerfile

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o billing-service ./cmd/billing

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/billing-service .
COPY --from=builder /app/configs/billing.yaml ./configs/
EXPOSE 8082 9092
CMD ["./billing-service"]
```
