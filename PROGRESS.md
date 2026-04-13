# MedCore — Progress Tracker

## Статус по шагам

| # | Шаг | Статус | Чат |
|---|---|---|---|
| 1 | Сбор требований с Astana Hub | ✅ Done | main |
| 2 | Промпты по каждому сервису | ✅ Done | main |
| 3 | Структура папок + go.mod | ✅ Done | main |
| 4 | shared/ пакеты | ✅ Done | main |
| 5 | auth-service | ✅ Done | main |
| 6 | billing-service | ✅ Done | main |
| 7 | integration-service | ✅ Done (11/11 tests) | chat-2 |
| 8 | analytics-service | ✅ Done (17/17 tests) | chat-3 |
| 9 | API Gateway | ✅ Done (7/7 tests) | main |
| 10 | docker-compose + Makefile | ✅ Done | main |
| 11 | README.md | ✅ Done (updated) | main |
| 12 | Текст предложения Astana Hub | ✅ Done | main |
| 13 | Frontend (web/) | ✅ Done (22/22 tests, build OK) | chat-4 |

## Pending fixes

- [x] `pkg/proto/auth/auth.proto` — сгенерированы auth.pb.go + auth_grpc.pb.go ✅
- [x] `migrations/` — 011 SQL файлов созданы (001-011) ✅
- [x] `internal/auth/handler/http.go` — `listUsers` реализован и покрыт тестом ✅
- [x] `internal/auth/handler/http.go` — `deactivateUser` вызывает service/repo цепочку и покрыт тестом ✅
- [x] `internal/billing/service/billing_service.go` — PDF генерируется через Gotenberg, nil panic убран, покрыто тестами ✅

## Актуальная валидация

- [x] `go test ./...` — backend тесты проходят ✅
- [x] `go test ./internal/auth/...` — auth handler/service тесты проходят ✅
- [x] `go test ./internal/billing/...` — billing handler/service тесты проходят ✅
- [x] `web: npm test` — frontend тесты проходят (`22/22`) ✅
- [x] `web: npm run build` — production build проходит ✅

## Архитектурные решения

| Решение | Выбор | Причина |
|---|---|---|
| PDF генерация | Gotenberg | HTML→PDF, брендинг клиник, качество |
| Kafka (Railway) | Upstash | Нет native Kafka на Railway |
| ClickHouse (Railway) | ClickHouse Cloud | Бесплатный tier |
| Production KZ | K8s на PS Cloud / VPS | ЗРК о персональных данных |
| Frontend | Vue 3 + Vite + ECharts | Современный стек, мощные графики |

## Порты сервисов

| Сервис | HTTP | gRPC |
|---|---|---|
| gateway | 8080 | — |
| auth | 8081 | 9091 |
| billing | 8082 | 9092 |
| integration | 8083 | 9093 |
| analytics | 8084 | 9094 |
| frontend (nginx) | 80 | — |
| gotenberg | 3000 | — |
