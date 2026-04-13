# Prompt 06 — Frontend (web/)

## Контекст проекта

Ты — Senior Frontend Developer. Разрабатываешь веб-интерфейс для платформы **MedCore** — медицинской SaaS платформы для казахстанского рынка (заказчик: ТОО Digital Clinic Hub, Astana Hub).

Backend уже реализован: Go микросервисы за API Gateway на `http://localhost:8080`.

---

## Стек

```
Framework:   Vue 3 + Vite
Language:    TypeScript
UI:          Tailwind CSS + Headless UI
Charts:      ECharts (vue-echarts)
HTTP:        axios + автоматический refresh токенов
State:       Pinia
Router:      Vue Router 4
Forms:       VeeValidate + Zod
i18n:        vue-i18n (русский + казахский)
Testing:     Vitest + Vue Test Utils
```

---

## Структура репозитория

```
web/
├── src/
│   ├── api/
│   │   ├── client.ts          # axios instance с interceptors
│   │   ├── auth.ts            # auth endpoints
│   │   ├── billing.ts         # billing endpoints
│   │   ├── analytics.ts       # analytics endpoints
│   │   └── integration.ts     # integration endpoints
│   ├── stores/
│   │   ├── auth.ts            # Pinia: user, tokens, login/logout
│   │   ├── billing.ts         # Pinia: invoices, subscriptions
│   │   └── analytics.ts       # Pinia: dashboard data
│   ├── router/
│   │   └── index.ts           # роуты + navigation guards
│   ├── views/
│   │   ├── auth/
│   │   │   ├── LoginView.vue
│   │   │   └── ChangePasswordView.vue
│   │   ├── dashboard/
│   │   │   └── DashboardView.vue      # главный дашборд
│   │   ├── analytics/
│   │   │   ├── RevenueView.vue        # выручка
│   │   │   ├── DoctorsView.vue        # загруженность врачей
│   │   │   └── ScheduleView.vue       # заполняемость расписания
│   │   ├── billing/
│   │   │   ├── InvoicesView.vue       # список счётов
│   │   │   ├── InvoiceDetailView.vue  # детали счёта + PDF
│   │   │   ├── PaymentView.vue        # оплата
│   │   │   └── SubscriptionView.vue   # тариф клиники
│   │   ├── users/
│   │   │   ├── UsersView.vue          # список пользователей [admin]
│   │   │   └── UserFormView.vue       # создание/редактирование
│   │   └── integration/
│   │       ├── IntegrationsView.vue   # настройки интеграций [admin]
│   │       └── LabResultsView.vue     # результаты анализов
│   ├── components/
│   │   ├── layout/
│   │   │   ├── AppLayout.vue          # sidebar + header
│   │   │   ├── Sidebar.vue
│   │   │   └── Header.vue
│   │   ├── charts/
│   │   │   ├── RevenueChart.vue       # ECharts: выручка по дням
│   │   │   ├── WorkloadChart.vue      # ECharts: загруженность врачей
│   │   │   └── FunnelChart.vue        # ECharts: воронка пациентов
│   │   ├── ui/
│   │   │   ├── BaseButton.vue
│   │   │   ├── BaseInput.vue
│   │   │   ├── BaseTable.vue          # с пагинацией
│   │   │   ├── BaseModal.vue
│   │   │   ├── BaseBadge.vue          # статусы платежей
│   │   │   └── LoadingSpinner.vue
│   │   └── billing/
│   │       ├── InvoiceCard.vue
│   │       └── PaymentStatusBadge.vue
│   ├── types/
│   │   ├── auth.ts            # User, Role, TokenPair
│   │   ├── billing.ts         # Invoice, Payment, Subscription, Plan
│   │   └── analytics.ts       # DoctorWorkload, ClinicRevenue, etc.
│   ├── composables/
│   │   ├── useAuth.ts         # хук аутентификации
│   │   ├── usePagination.ts   # общая пагинация
│   │   └── useExport.ts       # скачивание PDF/Excel
│   ├── utils/
│   │   ├── format.ts          # форматирование дат, денег (KZT)
│   │   └── permissions.ts     # проверка ролей/permissions
│   ├── locales/
│   │   ├── ru.json
│   │   └── kk.json
│   ├── App.vue
│   └── main.ts
├── index.html
├── vite.config.ts
├── tailwind.config.ts
├── tsconfig.json
└── package.json
```

---

## API Client

```typescript
// src/api/client.ts
// axios instance с базовым URL из env
// Request interceptor: добавляет Authorization: Bearer {access_token}
// Response interceptor:
//   - При 401: пробует refresh токен один раз
//   - Если refresh тоже 401: разлогинивает и редиректит на /login
//   - Прокидывает X-Request-ID для correlation

const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_URL || 'http://localhost:8080',
  timeout: 30000,
})
```

---

## TypeScript типы

```typescript
// src/types/auth.ts

export type Role = 'doctor' | 'coordinator' | 'admin' | 'super_admin'

export interface User {
  id: string
  clinic_id: string
  email: string
  first_name: string
  last_name: string
  phone: string
  role: Role
  is_active: boolean
  created_at: string
}

export interface TokenPair {
  access_token: string
  refresh_token: string
  expires_at: string
}

// src/types/billing.ts

export type InvoiceStatus = 'draft' | 'sent' | 'paid' | 'overdue' | 'voided'
export type PaymentStatus = 'pending' | 'processing' | 'completed' | 'failed' | 'refunded'
export type PlanTier = 'basic' | 'pro' | 'enterprise'

export interface Invoice {
  id: string
  clinic_id: string
  patient_id: string
  service_name: string
  amount: string
  currency: string
  status: InvoiceStatus
  due_at: string
  paid_at?: string
  pdf_url?: string
  created_at: string
}

export interface Plan {
  id: string
  tier: PlanTier
  name: string
  price_monthly: string
  currency: string
  max_doctors: number
  max_patients: number
  features: string[]
}

// src/types/analytics.ts

export interface DoctorWorkload {
  doctor_id: string
  doctor_name: string
  period: string
  total_appointments: number
  completed_count: number
  no_show_count: number
  no_show_rate: number
  workload_percent: number
}

export interface ClinicRevenue {
  clinic_id: string
  period: string
  total_revenue: number
  currency: string
  payment_count: number
  avg_check: number
  revenue_by_day: { date: string; revenue: number; count: number }[]
}
```

---

## Auth Store (Pinia)

```typescript
// src/stores/auth.ts
export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: null as User | null,
    accessToken: localStorage.getItem('access_token') || '',
    refreshToken: localStorage.getItem('refresh_token') || '',
  }),

  getters: {
    isAuthenticated: (state) => !!state.accessToken,
    isAdmin: (state) => ['admin', 'super_admin'].includes(state.user?.role ?? ''),
    isSuperAdmin: (state) => state.user?.role === 'super_admin',
    hasPermission: (state) => (perm: string) => {
      // проверяем по роли через матрицу прав
    },
  },

  actions: {
    async login(email: string, password: string): Promise<void>,
    async logout(): Promise<void>,
    async refreshTokens(): Promise<void>,
    async fetchMe(): Promise<void>,
  },
})
```

---

## Роутинг + Guards

```typescript
// src/router/index.ts

// Публичные роуты (без авторизации)
{ path: '/login', component: LoginView }

// Приватные роуты (требуют авторизации)
{ path: '/', component: AppLayout, children: [
  { path: 'dashboard', component: DashboardView },

  // Аналитика — только admin+
  { path: 'analytics/revenue',  component: RevenueView,  meta: { roles: ['admin', 'super_admin'] } },
  { path: 'analytics/doctors',  component: DoctorsView,  meta: { roles: ['admin', 'super_admin'] } },
  { path: 'analytics/schedule', component: ScheduleView, meta: { roles: ['admin', 'super_admin'] } },

  // Биллинг — admin и бухгалтер (coordinator)
  { path: 'billing/invoices',      component: InvoicesView },
  { path: 'billing/invoices/:id',  component: InvoiceDetailView },
  { path: 'billing/subscription',  component: SubscriptionView, meta: { roles: ['admin', 'super_admin'] } },

  // Пользователи — только admin+
  { path: 'users',     component: UsersView,    meta: { roles: ['admin', 'super_admin'] } },
  { path: 'users/new', component: UserFormView, meta: { roles: ['admin', 'super_admin'] } },

  // Интеграции — только admin+
  { path: 'integrations',  component: IntegrationsView, meta: { roles: ['admin', 'super_admin'] } },
  { path: 'lab-results',   component: LabResultsView },
]}

// Navigation guard:
// 1. Нет токена → /login
// 2. Роль не соответствует meta.roles → /dashboard с уведомлением "нет доступа"
```

---

## Dashboard View

```vue
<!-- Главный дашборд — одним запросом к GET /api/v1/dashboard -->
<!-- Показывает: -->
<!-- - Карточки KPI: выручка за месяц, кол-во пациентов, заполняемость, no-show rate -->
<!-- - График выручки по дням (ECharts Line) -->
<!-- - Топ врачей по загруженности (ECharts Bar) -->
<!-- - Текущий тарифный план с прогресс-баром (кол-во врачей / лимит) -->
<!-- - Последние 5 инвойсов -->

<!-- Требование из ТЗ: загрузка ≤ 2 секунды -->
<!-- Реализация: skeleton loader пока данные грузятся -->
```

---

## Charts (ECharts)

```typescript
// RevenueChart.vue — линейный график выручки по дням
// Ось X: дни месяца
// Ось Y: сумма в KZT
// Tooltip: дата + сумма + кол-во платежей
// Экспорт: кнопка "Скачать PNG" через ECharts saveAsImage

// WorkloadChart.vue — горизонтальный bar chart
// Ось Y: врачи
// Ось X: % загруженности
// Цвет: зелёный >80%, жёлтый 50-80%, красный <50%

// FunnelChart.vue — воронка конверсии пациентов
// Шаги: новый → первый визит → повторный → постоянный
```

---

## Billing страницы

### InvoicesView
```
- Таблица счётов с фильтрами: статус, период, поиск по пациенту
- Цветные badges по статусу (paid=зелёный, overdue=красный, pending=жёлтый)
- Кнопка "Скачать PDF" для каждого счёта
- Пагинация (20 на страницу)
```

### PaymentView
```
- Форма выбора провайдера (Kaspi Pay / Stripe)
- После создания — редирект на payment URL провайдера
- После возврата — показать статус платежа
```

### SubscriptionView
```
- Текущий тариф с фичами
- Прогресс использования (врачи, пациенты)
- Карточки тарифов Basic/Pro/Enterprise для апгрейда
- Кнопка отмены подписки с подтверждением
```

---

## Форматирование денег (KZT)

```typescript
// src/utils/format.ts

// Форматирование для казахстанского рынка
export function formatKZT(amount: number): string {
  return new Intl.NumberFormat('ru-KZ', {
    style: 'currency',
    currency: 'KZT',
    minimumFractionDigits: 0,
  }).format(amount)
  // → "49 900 ₸"
}

export function formatDate(date: string): string {
  return new Intl.DateTimeFormat('ru-KZ', {
    day: '2-digit', month: 'long', year: 'numeric',
  }).format(new Date(date))
  // → "15 апреля 2026"
}
```

---

## i18n (русский + казахский)

```json
// src/locales/ru.json
{
  "nav": {
    "dashboard": "Панель управления",
    "analytics": "Аналитика",
    "billing": "Биллинг",
    "users": "Пользователи",
    "integrations": "Интеграции"
  },
  "roles": {
    "doctor": "Врач",
    "coordinator": "Координатор",
    "admin": "Администратор",
    "super_admin": "Супер-администратор"
  },
  "invoice_status": {
    "paid": "Оплачен",
    "overdue": "Просрочен",
    "pending": "Ожидает оплаты",
    "draft": "Черновик"
  }
}
```

---

## Экспорт данных

```typescript
// src/composables/useExport.ts

// PDF инвойса: GET /api/v1/invoices/{id}/pdf → blob → скачать
// Excel аналитики: GET /api/v1/analytics/export/excel?... → blob → скачать
// PNG графика: ECharts saveAsImage action

async function downloadInvoicePDF(invoiceId: string): Promise<void> {
  const response = await apiClient.get(`/api/v1/invoices/${invoiceId}/pdf`, {
    responseType: 'blob',
  })
  const url = URL.createObjectURL(response.data)
  const a = document.createElement('a')
  a.href = url
  a.download = `invoice-${invoiceId}.pdf`
  a.click()
}
```

---

## Environment

```env
# web/.env.example
VITE_API_URL=http://localhost:8080
VITE_APP_NAME=MedCore
VITE_APP_ENV=development
```

---

## Docker

```dockerfile
# deployments/docker/web.Dockerfile

FROM node:20-alpine AS builder
WORKDIR /app
COPY web/package*.json ./
RUN npm ci
COPY web/ .
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY deployments/docker/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
```

```nginx
# deployments/docker/nginx.conf
server {
    listen 80;
    root /usr/share/nginx/html;
    index index.html;

    # SPA fallback
    location / {
        try_files $uri $uri/ /index.html;
    }

    # Proxy к API Gateway
    location /api/ {
        proxy_pass http://gateway:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

---

## Тесты

```
// Vitest + Vue Test Utils

// stores/auth.test.ts
test('login: сохраняет токены в localStorage')
test('logout: очищает store и localStorage')
test('refreshTokens: обновляет access token')

// composables/useAuth.test.ts
test('hasPermission: doctor не может видеть billing:manage')
test('hasPermission: admin может управлять пользователями')

// components/billing/InvoiceCard.test.ts
test('показывает правильный badge по статусу')
test('кнопка PDF вызывает downloadInvoicePDF')
```

---

## Важные принципы

- Роли контролируются на бэкенде — фронтенд только скрывает элементы интерфейса, не блокирует доступ
- Все суммы хранить как строки (не float) — использовать форматирование только при отображении
- Refresh токен в localStorage — access токен только в памяти (Pinia store)
- Skeleton loaders везде где есть async данные — UX требование
- Адаптивная вёрстка: desktop (admin панель) + tablet (врачи на планшетах в клинике)
- Error boundaries: при падении одного виджета дашборда остальные продолжают работать
- Все тексты через i18n — никаких хардкоженых русских строк в компонентах

---

## Порядок реализации

```
1. Настроить Vite + TypeScript + Tailwind + Pinia + Router
2. API client с interceptors (auth refresh)
3. Auth store + LoginView (это разблокирует всё остальное)
4. AppLayout (sidebar с навигацией по ролям)
5. DashboardView с KPI карточками
6. Analytics views с ECharts графиками
7. Billing views (инвойсы, оплата, подписка)
8. Users management (только admin)
9. Integrations & Lab results
10. i18n (ru + kk)
11. Тесты
12. Docker сборка
```

---

## Зависимости от backend

API Gateway доступен на `http://localhost:8080` (docker-compose).

Все endpoints задокументированы в промптах:
- Auth: `prompts/01-auth-service.md`
- Billing: `prompts/02-billing-service.md`
- Integration: `prompts/03-integration-service.md`
- Analytics: `prompts/04-analytics-service.md`

*Часть платформы MedCore | Автор: Nurtilek Assankhan | github.com/nurtidev*
