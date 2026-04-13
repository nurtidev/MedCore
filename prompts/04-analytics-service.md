# Prompt 04 — analytics-service (BI + KPI)

## Источник требований
- ТЗ: `docs/requirements/05-analytics.md`
- Заказчик: ТОО DIGITAL CLINIC HUB

## Контекст

Разрабатываешь `analytics-service` для платформы **MedCore**.

Из реального ТЗ заказчика: *"обеспечить руководителей клиник прозрачной аналитикой для принятия управленческих решений"*. Цели: сокращение времени администрирования на **35–50%**, заполняемость расписания до **85–90%**. Время загрузки дашбордов: **≤ 2 секунды**.

Сервис потребляет события из Kafka (от auth, billing, integration), хранит в ClickHouse, предоставляет REST API для дашбордов.

**Зависимость:** использует `auth-service` через gRPC для валидации токенов.

---

## Структура файлов

```
cmd/analytics/
└── main.go

internal/analytics/
├── domain/
│   ├── event.go         # ClinicEvent — базовая единица аналитики
│   ├── kpi.go           # KPI structs: DoctorWorkload, ClinicRevenue, etc.
│   └── errors.go
├── repository/
│   ├── clickhouse_repo.go        # interface
│   └── clickhouse_repo_impl.go   # clickhouse-go/v2
├── service/
│   ├── analytics_service.go
│   └── analytics_service_test.go
├── handler/
│   ├── http.go
│   └── http_test.go
└── worker/
    ├── kafka_consumer.go  # читает события из всех сервисов
    └── cron_worker.go     # пересчёт агрегатов по расписанию

configs/analytics.yaml
deployments/docker/analytics.Dockerfile
```

---

## Domain

```go
// internal/analytics/domain/event.go

type EventType string
const (
    EventAppointmentCreated   EventType = "appointment.created"
    EventAppointmentCompleted EventType = "appointment.completed"
    EventAppointmentNoShow    EventType = "appointment.no_show"
    EventAppointmentCancelled EventType = "appointment.cancelled"
    EventPaymentCompleted     EventType = "payment.completed"
    EventPaymentFailed        EventType = "payment.failed"
    EventLabResultReceived    EventType = "lab_result.received"
    EventUserLogin            EventType = "user.login"
)

type ClinicEvent struct {
    EventID    string    // UUID
    ClinicID   string
    DoctorID   string
    PatientID  string
    EventType  EventType
    Amount     float64   // для payment событий
    Currency   string
    CreatedAt  time.Time
    Metadata   string    // JSON
}

// internal/analytics/domain/kpi.go

type DoctorWorkload struct {
    DoctorID          string
    DoctorName        string
    Period            string    // "2025-04"
    TotalAppointments int64
    CompletedCount    int64
    NoShowCount       int64
    CancelledCount    int64
    WorkloadPercent   float64   // completed / scheduled slots
    NoShowRate        float64   // no_show / total
}

type ClinicRevenue struct {
    ClinicID      string
    Period        string
    TotalRevenue  float64
    Currency      string
    PaymentCount  int64
    AvgCheck      float64
    RevenueByDay  []DailyRevenue
}

type DailyRevenue struct {
    Date    string
    Revenue float64
    Count   int64
}

type ScheduleFillRate struct {
    ClinicID       string
    Period         string
    TotalSlots     int64
    FilledSlots    int64
    FillRatePercent float64
}

type PatientFunnel struct {
    ClinicID          string
    Period            string
    NewPatients       int64
    ReturnPatients    int64
    RetentionRate     float64
}
```

---

## ClickHouse Schema

```sql
-- Основная таблица событий
CREATE TABLE clinic_events (
    event_id    UUID,
    clinic_id   UUID,
    doctor_id   UUID,
    patient_id  UUID,
    event_type  String,
    amount      Float64,
    currency    String,
    created_at  DateTime,
    metadata    String    -- JSON
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(created_at)
ORDER BY (clinic_id, created_at)
TTL created_at + INTERVAL 3 YEAR;

-- Materialized View: загруженность врачей по месяцам
CREATE MATERIALIZED VIEW doctor_workload_mv
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(period)
ORDER BY (clinic_id, doctor_id, period)
AS SELECT
    clinic_id,
    doctor_id,
    toStartOfMonth(created_at) AS period,
    countIf(event_type = 'appointment.created')   AS total_appointments,
    countIf(event_type = 'appointment.completed') AS completed_count,
    countIf(event_type = 'appointment.no_show')   AS no_show_count,
    countIf(event_type = 'appointment.cancelled') AS cancelled_count
FROM clinic_events
GROUP BY clinic_id, doctor_id, period;

-- Materialized View: выручка клиники по дням
CREATE MATERIALIZED VIEW clinic_revenue_mv
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(period)
ORDER BY (clinic_id, period, currency)
AS SELECT
    clinic_id,
    toStartOfDay(created_at) AS period,
    currency,
    sumIf(amount, event_type = 'payment.completed') AS total_revenue,
    countIf(event_type = 'payment.completed')       AS payment_count
FROM clinic_events
GROUP BY clinic_id, period, currency;

-- Materialized View: заполняемость расписания
CREATE MATERIALIZED VIEW schedule_fill_mv
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(period)
ORDER BY (clinic_id, period)
AS SELECT
    clinic_id,
    toStartOfMonth(created_at) AS period,
    countIf(event_type = 'appointment.created')   AS total_slots,
    countIf(event_type = 'appointment.completed') AS filled_slots
FROM clinic_events
GROUP BY clinic_id, period;
```

---

## Service Interface

```go
type AnalyticsService interface {
    // KPI дашборды (≤ 2 секунды — требование из ТЗ)
    GetDoctorWorkload(ctx context.Context, req WorkloadRequest) ([]*DoctorWorkload, error)
    GetClinicRevenue(ctx context.Context, req RevenueRequest) (*ClinicRevenue, error)
    GetScheduleFillRate(ctx context.Context, req FillRateRequest) (*ScheduleFillRate, error)
    GetPatientFunnel(ctx context.Context, req FunnelRequest) (*PatientFunnel, error)

    // Сводный дашборд (все KPI за период одним запросом)
    GetDashboard(ctx context.Context, clinicID uuid.UUID, period string) (*Dashboard, error)

    // Экспорт (из ТЗ: PDF/Excel)
    ExportToExcel(ctx context.Context, req ExportRequest) ([]byte, error)
    ExportToCSV(ctx context.Context, req ExportRequest) ([]byte, error)

    // Запись событий (внутренний вызов от Kafka consumer)
    RecordEvent(ctx context.Context, event *ClinicEvent) error
    RecordEventBatch(ctx context.Context, events []*ClinicEvent) error
}

type WorkloadRequest struct {
    ClinicID uuid.UUID
    Period   string      // "2025-04" или диапазон
    DoctorID *uuid.UUID  // если nil — все врачи
}

type RevenueRequest struct {
    ClinicID  uuid.UUID
    StartDate time.Time
    EndDate   time.Time
    Grouping  string    // "day", "week", "month"
}
```

---

## Kafka Consumer

```go
// internal/analytics/worker/kafka_consumer.go
// Читает из топиков:
//   "payment.completed"              → EventPaymentCompleted
//   "payment.failed"                 → EventPaymentFailed
//   "integration.appointment.created" → EventAppointmentCreated
//   "integration.lab_result.received" → EventLabResultReceived
//   "user.login"                     → EventUserLogin (из audit log auth-service)
//
// Для каждого события:
// 1. Парсить JSON payload
// 2. Маппировать в ClinicEvent
// 3. Batch insert в ClickHouse (каждые 100 событий или каждые 5 секунд)
// 4. При ошибке ClickHouse — не коммитить offset, retry
//
// Consumer group: "analytics-service"
// Batch size: 100 событий
// Flush interval: 5s
```

---

## REST API

```
# Дашборды (из ТЗ: дневные/недельные/месячные)
GET /api/v1/analytics/dashboard?clinic_id=&period=2025-04

# KPI метрики
GET /api/v1/analytics/doctors/workload?clinic_id=&period=&doctor_id=
GET /api/v1/analytics/revenue?clinic_id=&start=&end=&grouping=day
GET /api/v1/analytics/schedule/fill-rate?clinic_id=&period=
GET /api/v1/analytics/patients/funnel?clinic_id=&period=

# Экспорт (из ТЗ: PDF/Excel)
GET /api/v1/analytics/export/excel?clinic_id=&period=&type=revenue
GET /api/v1/analytics/export/csv?clinic_id=&period=&type=workload

# Синхронизация с внешними BI системами (из ТЗ)
GET /api/v1/analytics/bi-sync?clinic_id=&from=&to=  # JSON для внешних BI

GET /health
GET /ready
```

**Роли согласно ТЗ:**
- `super_admin` — аналитика по всем клиникам (SaaS уровень)
- `admin` — только своя клиника
- `doctor` — только свои показатели

---

## CRON задачи

```go
// internal/analytics/worker/cron_worker.go

// Каждый час: инвалидировать кэш дашбордов (Redis TTL = 1h)
// Каждую ночь в 02:00: пересчитать агрегаты за предыдущий день
// Каждое воскресенье: генерировать недельный отчёт для клиник на Pro/Enterprise плане
```

---

## Redis кэш для дашбордов

```go
// Ключ: "analytics:dashboard:{clinic_id}:{period}"
// TTL: 1 час (данные не меняются часто, но должны быть свежими)
//
// Ключ: "analytics:revenue:{clinic_id}:{start}:{end}:{grouping}"
// TTL: 30 минут
//
// При RecordEvent — инвалидировать кэш для clinic_id
```

---

## Prometheus метрики

```go
analytics_events_ingested_total{event_type}   // счётчик событий
analytics_query_duration_seconds{query_type}  // гистограмма запросов к ClickHouse
analytics_clickhouse_batch_size               // размер батча
analytics_kafka_consumer_lag                  // отставание consumer
```

---

## Конфигурация

```yaml
# configs/analytics.yaml
server:
  http_port: 8084
  grpc_port: 9094

clickhouse:
  dsn: "${CLICKHOUSE_DSN}"
  max_open_conns: 10
  dial_timeout: "10s"

redis:
  addr: "${REDIS_ADDR}"

kafka:
  brokers: "${KAFKA_BROKERS}"
  group_id: "analytics-service"
  topics:
    - "payment.completed"
    - "payment.failed"
    - "integration.appointment.created"
    - "integration.lab_result.received"

consumer:
  batch_size: 100
  flush_interval: "5s"

auth_grpc:
  addr: "auth-service:9091"

log:
  level: "info"
  format: "json"
```

---

## Тесты

```
// service/analytics_service_test.go
TestGetDoctorWorkload_Success
TestGetDoctorWorkload_EmptyPeriod
TestGetClinicRevenue_GroupByDay
TestGetClinicRevenue_GroupByMonth
TestGetDashboard_UnderTwoSeconds   // performance test
TestRecordEventBatch_Success
TestExportToExcel_Success

// worker/kafka_consumer_test.go
TestKafkaConsumer_PaymentCompleted_RecordsEvent
TestKafkaConsumer_BatchFlush_On100Events
TestKafkaConsumer_BatchFlush_OnTimeout
```

---

## Dockerfile

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o analytics-service ./cmd/analytics

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/analytics-service .
COPY --from=builder /app/configs/analytics.yaml ./configs/
EXPOSE 8084 9094
CMD ["./analytics-service"]
```

---

## Важные принципы

- ClickHouse — только для чтения аналитики и записи событий, не для транзакций
- Batch insert в ClickHouse — никогда не вставлять по одной строке
- Materialized views — пре-агрегация для соблюдения ≤2s на дашборды
- Redis кэш обязателен — прямые запросы к ClickHouse на каждый HTTP вызов недопустимы
- Kafka consumer group offset коммитить только после успешного insert в ClickHouse
- Партиционирование по месяцам — запросы должны попадать в нужную партицию (не делать full scan)
