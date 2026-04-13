# Prompt 03 — integration-service (ГосAPI + Лаборатории + Агрегаторы)

## Источник требований
- ТЗ ГосAPI: `docs/requirements/03-gosapi.md`
- ТЗ Интеграции: `docs/requirements/04-integrations.md`
- Заказчик: ТОО DIGITAL CLINIC HUB

## Контекст

Разрабатываешь `integration-service` для платформы **MedCore**.

Этот сервис закрывает **два ТЗ заказчика** одновременно:
1. *"Подключение к внешним медицинским базам"* — eGov API, DAMUMED (ГосAPI)
2. *"Автоматический обмен данными между DCH и сторонними сервисами"* — iDoctor, лаборатории Олимп/Инвиво

Ключевое требование: задержка появления записи ≤ **1 секунды**, снижение ручного труда на **35–50%**.

**Зависимость:** использует `auth-service` через gRPC для валидации токенов.

---

## Структура файлов

```
cmd/integration/
└── main.go

internal/integration/
├── domain/
│   ├── patient.go        # PatientInfo из ГосAPI
│   ├── appointment.go    # Appointment из агрегаторов
│   ├── lab_result.go     # LabResult от лабораторий
│   └── errors.go
├── repository/
│   ├── sync_repo.go
│   ├── postgres_sync_repo.go
│   ├── lab_result_repo.go
│   └── postgres_lab_result_repo.go
├── service/
│   ├── integration_service.go
│   └── integration_service_test.go
├── handler/
│   ├── http.go           # internal API endpoints
│   ├── http_test.go
│   └── webhook.go        # входящие webhooks от агрегаторов/лабораторий
├── adapter/
│   ├── adapter.go        # interface ExternalAdapter
│   ├── egov/
│   │   └── egov_client.go     # eGov API (ИИН валидация, данные граждан)
│   ├── damumed/
│   │   └── damumed_client.go  # DAMUMED (медреестры РК)
│   ├── idoctor/
│   │   └── idoctor_client.go  # агрегатор записей
│   ├── olymp/
│   │   └── olymp_client.go    # лаборатория Олимп
│   └── invivo/
│       └── invivo_client.go   # лаборатория Инвиво
└── worker/
    ├── sync_worker.go    # фоновая синхронизация расписания
    └── kafka_consumer.go # слушает события из других сервисов

migrations/
├── 009_create_integration_configs.sql
├── 010_create_sync_logs.sql
└── 011_create_lab_results.sql

configs/integration.yaml
deployments/docker/integration.Dockerfile
```

---

## Domain

```go
// internal/integration/domain/patient.go
// Данные пациента из eGov API

type PatientInfo struct {
    IIN        string
    FirstName  string
    LastName   string
    MiddleName string
    BirthDate  time.Time
    Gender     string
    Address    string
    IsValid    bool   // ИИН существует в базе
}

// internal/integration/domain/appointment.go
// Запись из внешнего агрегатора (iDoctor)

type ExternalAppointment struct {
    ExternalID     string
    ExternalSource string    // "idoctor"
    DoctorID       string    // внешний ID врача
    InternalDoctorID *uuid.UUID // маппинг на внутренний ID
    PatientName    string
    PatientPhone   string
    PatientIIN     string
    ServiceName    string
    ScheduledAt    time.Time
    Status         string    // "booked", "cancelled", "completed"
    CreatedAt      time.Time
}

// internal/integration/domain/lab_result.go

type LabResultFormat string
const (
    FormatPDF  LabResultFormat = "pdf"
    FormatJSON LabResultFormat = "json"
    FormatXML  LabResultFormat = "xml"
)

type LabResult struct {
    ID           uuid.UUID
    ClinicID     uuid.UUID
    PatientID    uuid.UUID
    ExternalID   string
    LabProvider  string          // "olymp", "invivo"
    TestName     string
    Format       LabResultFormat
    FileURL      string          // для PDF
    Data         map[string]any  // для JSON/XML
    ReceivedAt   time.Time
    AttachedAt   *time.Time      // когда прикреплено к карте
}
```

---

## Migrations (Goose)

### 009_create_integration_configs.sql
```sql
-- +goose Up
CREATE TABLE integration_configs (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    clinic_id    UUID NOT NULL,
    provider     VARCHAR(100) NOT NULL,   -- "idoctor", "olymp", "invivo"
    is_active    BOOLEAN NOT NULL DEFAULT true,
    config       JSONB NOT NULL,           -- API ключи, URL (зашифрованы)
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(clinic_id, provider)
);

-- +goose Down
DROP TABLE integration_configs;
```

### 010_create_sync_logs.sql
```sql
-- +goose Up
CREATE TABLE sync_logs (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    clinic_id    UUID NOT NULL,
    provider     VARCHAR(100) NOT NULL,
    operation    VARCHAR(100) NOT NULL,   -- "sync_appointments", "fetch_lab_result"
    status       VARCHAR(50) NOT NULL,    -- "success", "failed", "partial"
    records_processed INT DEFAULT 0,
    error_message TEXT,
    started_at   TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sync_logs_clinic_id ON sync_logs(clinic_id);
CREATE INDEX idx_sync_logs_provider ON sync_logs(provider);
CREATE INDEX idx_sync_logs_created_at ON sync_logs(created_at);

-- +goose Down
DROP TABLE sync_logs;
```

### 011_create_lab_results.sql
```sql
-- +goose Up
CREATE TABLE lab_results (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    clinic_id    UUID NOT NULL,
    patient_id   UUID NOT NULL,
    external_id  VARCHAR(255),
    lab_provider VARCHAR(100) NOT NULL,
    test_name    VARCHAR(255) NOT NULL,
    format       VARCHAR(20) NOT NULL,
    file_url     VARCHAR(500),
    data         JSONB,
    received_at  TIMESTAMPTZ NOT NULL,
    attached_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_lab_results_patient_id ON lab_results(patient_id);
CREATE INDEX idx_lab_results_clinic_id ON lab_results(clinic_id);
CREATE INDEX idx_lab_results_received_at ON lab_results(received_at);

-- +goose Down
DROP TABLE lab_results;
```

---

## Adapter Interface

```go
// internal/integration/adapter/adapter.go
// Единый интерфейс для всех внешних провайдеров

type GovAPIAdapter interface {
    ValidateIIN(ctx context.Context, iin string) (*PatientInfo, error)
    GetPatientStatus(ctx context.Context, iin string) (string, error)
}

type AggregatorAdapter interface {
    GetNewAppointments(ctx context.Context, clinicID string, since time.Time) ([]*ExternalAppointment, error)
    UpdateAppointmentStatus(ctx context.Context, externalID, status string) error
    GetDoctorMapping(ctx context.Context, clinicID string) (map[string]uuid.UUID, error)
}

type LaboratoryAdapter interface {
    GetPendingResults(ctx context.Context, clinicID string) ([]*LabResult, error)
    AcknowledgeResult(ctx context.Context, externalID string) error
}
```

---

## Circuit Breaker + Retry

```go
// Каждый adapter оборачивать в:
// 1. Circuit breaker (sony/gobreaker):
//    - MaxRequests: 3 (в half-open)
//    - Interval: 60s
//    - Timeout: 30s (время в open state)
//    - ReadyToTrip: 5 consecutive failures
//
// 2. Retry с exponential backoff (avast/retry-go):
//    - Attempts: 3
//    - Delay: 100ms → 200ms → 400ms
//    - RetryIf: только на сетевые ошибки и 5xx, НЕ на 4xx
//
// 3. Timeout на каждый внешний вызов: 10s через context.WithTimeout
```

---

## Redis кэширование

```go
// Стратегия кэша по типу данных:
// IIN валидация (eGov)     → TTL 24h  (данные меняются редко)
// Статус пациента (DAMUMED) → TTL 1h
// Маппинг ID врачей        → TTL 6h
// Конфиги интеграций       → TTL 5m
//
// Ключи: "integration:{provider}:{operation}:{param}"
// Пример: "integration:egov:iin:860101123456"
```

---

## Service Interface

```go
type IntegrationService interface {
    // ГосAPI
    ValidateIIN(ctx context.Context, iin string) (*PatientInfo, error)
    GetPatientStatus(ctx context.Context, iin string) (string, error)

    // Агрегаторы
    SyncAppointments(ctx context.Context, clinicID uuid.UUID) (*SyncResult, error)
    HandleIncomingAppointment(ctx context.Context, payload WebhookPayload) error

    // Лаборатории
    FetchLabResults(ctx context.Context, clinicID uuid.UUID) ([]*LabResult, error)
    AttachResultToPatient(ctx context.Context, resultID uuid.UUID, patientID uuid.UUID) error
    HandleLabWebhook(ctx context.Context, provider string, payload []byte) error

    // Конфиги
    GetIntegrationConfig(ctx context.Context, clinicID uuid.UUID, provider string) (*IntegrationConfig, error)
    UpsertIntegrationConfig(ctx context.Context, req UpsertConfigRequest) error
}
```

---

## REST API

```
# ГосAPI
POST   /api/v1/gov/validate-iin          # валидация ИИН [auth required]
GET    /api/v1/gov/patient-status/{iin}  # статус пациента [auth required]

# Агрегаторы
POST   /api/v1/sync/appointments/{clinic_id}  # ручной запуск синхронизации [admin]
GET    /api/v1/sync/logs/{clinic_id}          # история синхронизаций [admin]

# Лаборатории
GET    /api/v1/lab-results/{clinic_id}             # список результатов [admin/doctor]
POST   /api/v1/lab-results/{id}/attach/{patient_id} # прикрепить к карте [admin]

# Конфиги интеграций
GET    /api/v1/integrations/{clinic_id}        # список интеграций [admin]
PUT    /api/v1/integrations/{clinic_id}/{provider} # настроить интеграцию [admin]

# Входящие webhooks (без auth — проверка подписи внутри)
POST   /webhooks/idoctor                  # новая запись от iDoctor
POST   /webhooks/olymp                    # результат от лаборатории Олимп
POST   /webhooks/invivo                   # результат от лаборатории Инвиво

GET    /health
GET    /ready
```

---

## Фоновый воркер синхронизации

```go
// internal/integration/worker/sync_worker.go
// Каждые 30 секунд для каждой активной клиники с настроенным iDoctor:
// 1. Получить новые записи с момента последней синхронизации
// 2. Сделать маппинг external_doctor_id → internal UUID
// 3. Создать appointment в DCH через внутренний API
// 4. Обновить статус в iDoctor
// 5. Записать sync_log
// 6. Время выполнения цикла ≤ 1 секунды (требование из ТЗ)
```

---

## Dead Letter Queue

```go
// При неуспешной интеграции (после всех retry):
// → Отправить сообщение в Kafka топик "integration.failed"
// → Структура: {provider, operation, payload, error, attempts, failed_at}
// → Отдельный consumer читает DLQ и алертит или повторяет через N минут
```

---

## Kafka Events

```go
// Продюсирует:
// "integration.appointment.created" — новая запись из агрегатора
// "integration.lab_result.received" — результат от лаборатории
// "integration.failed"              — DLQ для неуспешных операций

// Потребляет:
// "payment.completed" — для обновления статуса услуги в агрегаторе
```

---

## Конфигурация

```yaml
# configs/integration.yaml
server:
  http_port: 8083
  grpc_port: 9093

database:
  dsn: "${DATABASE_URL}"

redis:
  addr: "${REDIS_ADDR}"

kafka:
  brokers: "${KAFKA_BROKERS}"

egov:
  api_url: "${EGOV_API_URL}"
  api_key: "${EGOV_API_KEY}"
  timeout: "10s"

damumed:
  api_url: "${DAMUMED_API_URL}"
  api_key: "${DAMUMED_API_KEY}"

idoctor:
  api_url: "${IDOCTOR_API_URL}"
  webhook_secret: "${IDOCTOR_WEBHOOK_SECRET}"

olymp_lab:
  api_url: "${OLYMP_API_URL}"
  api_key: "${OLYMP_API_KEY}"

invivo_lab:
  api_url: "${INVIVO_API_URL}"
  api_key: "${INVIVO_API_KEY}"

sync:
  interval: "30s"

auth_grpc:
  addr: "auth-service:9091"

log:
  level: "info"
  format: "json"
```

---

## Тесты

```
// service/integration_service_test.go
TestValidateIIN_Success
TestValidateIIN_InvalidIIN
TestValidateIIN_CacheHit
TestSyncAppointments_Success
TestSyncAppointments_CircuitBreakerOpen
TestHandleLabWebhook_Olymp_Success
TestHandleLabWebhook_InvalidSignature

// adapter/egov/egov_client_test.go
TestEgovClient_ValidateIIN_NetworkError_Retry
TestEgovClient_5xx_CircuitBreakerTrips
```

---

## Dockerfile

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o integration-service ./cmd/integration

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/integration-service .
COPY --from=builder /app/configs/integration.yaml ./configs/
EXPOSE 8083 9093
CMD ["./integration-service"]
```

---

## Важные принципы

- Adapter per provider — никакого if/switch по провайдеру в бизнес-логике
- Все внешние вызовы через circuit breaker + retry — без исключений
- Конфиги интеграций (API ключи) хранить зашифрованными в БД
- Логировать каждый внешний вызов: provider, duration, status
- Webhook входящие: всегда проверять подпись ДО обработки payload
- Ответ на webhook: 200 OK немедленно, обработка асинхронно через Kafka
