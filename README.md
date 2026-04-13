# MedCore

MedCore is a medical SaaS platform for the Kazakhstan market built as a Go microservice monorepo.
The project covers authentication and RBAC, billing, external medical integrations, analytics, API gateway routing, and a Vue 3 frontend.

## What Is Included

- `auth-service`: JWT auth, refresh tokens, RBAC, user management, encrypted IIN storage
- `billing-service`: Kaspi Pay and Stripe payment links, invoices, subscriptions, webhook processing, PDF generation via Gotenberg
- `integration-service`: eGov, DAMUMED, iDoctor, Olymp, Invivo adapters, inbound webhooks, sync workers
- `analytics-service`: KPI APIs, ClickHouse-backed analytics, Kafka consumer, scheduled recalculation jobs
- `gateway`: single public entrypoint, auth validation via gRPC, rate limiting, CORS, upstream aggregation
- `web`: Vue 3 dashboard with billing, analytics, integrations, users, and auth flows

## Architecture

```text
                        +-------------------+
Browser / Frontend ---> |     Gateway       | :8080
                        +---------+---------+
                                  |
          +-----------------------+-----------------------+
          |                       |                       |
   +------+-------+        +------+-------+        +------+-------+
   |     Auth     |        |    Billing   |        | Integration  |
   |  :8081/:9091 |        |  :8082/:9092 |        |  :8083/:9093 |
   +------+-------+        +------+-------+        +------+-------+
          |                       |                       |
          +-----------+-----------+-----------+-----------+
                      |                       |
                +-----+------+         +------+------+
                | PostgreSQL |         |    Redis    |
                +------------+         +-------------+

                +------------+         +-------------+         +------------+
                |   Kafka    |         | ClickHouse  |         | Gotenberg  |
                +------------+         +-------------+         +------------+

                                +-------------------+
                                |    Analytics      | :8084/:9094
                                +-------------------+
```

## Services

| Component | Ports | Purpose |
|---|---|---|
| `gateway` | `8080` | Public HTTP entrypoint, JWT validation, proxying, dashboard aggregation |
| `auth-service` | `8081`, `9091` | Auth, RBAC, users, token lifecycle |
| `billing-service` | `8082`, `9092` | Payments, invoices, subscriptions, PDF invoices |
| `integration-service` | `8083`, `9093` | State and lab integrations, sync jobs, partner webhooks |
| `analytics-service` | `8084`, `9094` | KPI APIs, event ingestion, scheduled analytics |
| `web` | `80` | Nginx-served frontend for local full-stack run |
| `gotenberg` | `3000` | HTML-to-PDF conversion service |

## Tech Stack

| Layer | Stack |
|---|---|
| Backend | Go 1.22, chi, gRPC, pgx, Zerolog, Viper |
| Data | PostgreSQL, Redis, ClickHouse |
| Messaging | Kafka |
| Payments | Kaspi Pay, Stripe |
| PDF | Gotenberg 8 |
| Frontend | Vue 3, Vite, TypeScript, Pinia, Vue Router, Vue I18n, ECharts |
| Infra | Docker Compose, Dockerfiles, Kubernetes-ready deployment layout |
| Testing | Go test, Testify, Vitest, Vue Test Utils |

## Current Status

- Backend modules are implemented and passing tests
- Frontend is implemented, builds successfully, and its test suite passes
- Invoice PDF generation is wired to Gotenberg
- Core technical and documentation deliverables in the progress tracker are completed

Current validated commands:

```bash
go test ./...
go test ./internal/auth/...
go test ./internal/billing/...
cd web && npm test
cd web && npm run build
```

## Quick Start

### Prerequisites

- Go `1.22+`
- Node.js `20+`
- npm
- Docker + Docker Compose v2
- `make`

### 1. Create Environment File

```bash
make env
```

Then review and update `.env`:

- `JWT_SECRET`
- `IIN_ENCRYPTION_KEY`
- payment provider secrets
- external integration keys

See [.env.example](.env.example).

### 2. Start the Full Stack

```bash
make up
```

Local endpoints after startup:

- Frontend: `http://localhost`
- Gateway: `http://localhost:8080`
- Gotenberg: `http://localhost:3000`

### 3. Health Check

```bash
curl http://localhost:8080/health
```

## Local Development

### Infrastructure Only

```bash
make up-infra
```

Then run services individually, for example:

```bash
go run ./cmd/auth
go run ./cmd/billing
go run ./cmd/integration
go run ./cmd/analytics
go run ./cmd/gateway
```

### Frontend Only

```bash
cd web
npm install
npm run dev
```

Default Vite dev server: `http://localhost:5173`

## Make Targets

Common commands:

```bash
make up
make down
make down-v
make ps
make logs
make logs-gateway
make build
make test
make test-auth
make test-billing
make cover
make fmt
make vet
make lint
make proto
make migrate-up
make migrate-status
make help
```

## API Overview

Public routes via gateway:

```text
POST /api/v1/auth/login
POST /api/v1/auth/register
POST /api/v1/auth/refresh
GET  /api/v1/plans
POST /webhooks/kaspi
POST /webhooks/stripe
POST /webhooks/idoctor
POST /webhooks/olymp
POST /webhooks/invivo
GET  /health
GET  /ready
```

Protected route groups via gateway:

```text
GET  /api/v1/dashboard
/api/v1/users/*
/api/v1/payments/*
/api/v1/invoices/*
/api/v1/subscriptions/*
/api/v1/gov/*
/api/v1/sync/*
/api/v1/lab-results/*
/api/v1/integrations/*
/api/v1/analytics/*
```

## Repository Layout

```text
medcore/
├── cmd/                 # service entrypoints
├── configs/             # YAML service configs
├── deployments/         # Dockerfiles and deployment assets
├── docs/                # requirements and proposal docs
├── internal/
│   ├── analytics/
│   ├── auth/
│   ├── billing/
│   ├── gateway/
│   ├── integration/
│   └── shared/
├── migrations/          # SQL schema migrations
├── pkg/proto/           # generated protobuf files
├── prompts/             # implementation prompts by module
├── web/                 # Vue frontend
├── docker-compose.yml
├── Makefile
└── PROGRESS.md
```

## Environment Variables

Key variables from [.env.example](.env.example):

| Variable | Meaning |
|---|---|
| `JWT_SECRET` | JWT signing secret |
| `IIN_ENCRYPTION_KEY` | AES key for IIN encryption |
| `KASPI_API_URL` | Kaspi API base URL |
| `KASPI_MERCHANT_ID` | Kaspi merchant identifier |
| `KASPI_SECRET_KEY` | Kaspi webhook/API secret |
| `STRIPE_SECRET_KEY` | Stripe secret key |
| `STRIPE_WEBHOOK_SECRET` | Stripe webhook secret |
| `EGOV_API_KEY` | eGov access key |
| `DAMUMED_API_KEY` | DAMUMED access key |
| `IDOCTOR_WEBHOOK_SECRET` | iDoctor webhook secret |
| `OLYMP_API_KEY` | Olymp Lab API key |
| `INVIVO_API_KEY` | Invivo Lab API key |

## Notes

- Personal medical data handling is designed with Kazakhstan data-protection requirements in mind
- The gateway is the intended public entrypoint for client applications
- Billing PDF generation depends on the `gotenberg` service being available
- Frontend localization currently includes Russian and Kazakh dictionaries

## Related Docs

- [Progress Tracker](PROGRESS.md)
- [Astana Hub Proposal](docs/astana-hub-proposal.md)
- [Auth Requirements](docs/requirements/01-auth-rbac.md)
- [Billing Requirements](docs/requirements/02-billing.md)
- [GosAPI Requirements](docs/requirements/03-gosapi.md)
- [Integrations Requirements](docs/requirements/04-integrations.md)
- [Analytics Requirements](docs/requirements/05-analytics.md)
