import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import type { Role } from '@/types/auth'

declare module 'vue-router' {
  interface RouteMeta {
    requiresAuth?: boolean
    roles?: Role[]
  }
}

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/auth/LoginView.vue'),
    },
    {
      path: '/',
      component: () => import('@/components/layout/AppLayout.vue'),
      meta: { requiresAuth: true },
      children: [
        {
          path: '',
          redirect: '/dashboard',
        },
        {
          path: 'dashboard',
          name: 'dashboard',
          component: () => import('@/views/dashboard/DashboardView.vue'),
        },
        // Analytics — admin+
        {
          path: 'analytics/revenue',
          name: 'analytics-revenue',
          component: () => import('@/views/analytics/RevenueView.vue'),
          meta: { roles: ['admin', 'super_admin'] },
        },
        {
          path: 'analytics/doctors',
          name: 'analytics-doctors',
          component: () => import('@/views/analytics/DoctorsView.vue'),
          meta: { roles: ['admin', 'super_admin'] },
        },
        {
          path: 'analytics/schedule',
          name: 'analytics-schedule',
          component: () => import('@/views/analytics/ScheduleView.vue'),
          meta: { roles: ['admin', 'super_admin'] },
        },
        // Billing
        {
          path: 'billing/invoices',
          name: 'invoices',
          component: () => import('@/views/billing/InvoicesView.vue'),
        },
        {
          path: 'billing/invoices/:id',
          name: 'invoice-detail',
          component: () => import('@/views/billing/InvoiceDetailView.vue'),
        },
        {
          path: 'billing/payment/:invoiceId',
          name: 'payment',
          component: () => import('@/views/billing/PaymentView.vue'),
        },
        {
          path: 'billing/subscription',
          name: 'subscription',
          component: () => import('@/views/billing/SubscriptionView.vue'),
          meta: { roles: ['admin', 'super_admin'] },
        },
        // Users — admin+
        {
          path: 'users',
          name: 'users',
          component: () => import('@/views/users/UsersView.vue'),
          meta: { roles: ['admin', 'super_admin'] },
        },
        {
          path: 'users/new',
          name: 'user-new',
          component: () => import('@/views/users/UserFormView.vue'),
          meta: { roles: ['admin', 'super_admin'] },
        },
        {
          path: 'users/:id/edit',
          name: 'user-edit',
          component: () => import('@/views/users/UserFormView.vue'),
          meta: { roles: ['admin', 'super_admin'] },
        },
        // Account
        {
          path: 'account/change-password',
          name: 'change-password',
          component: () => import('@/views/auth/ChangePasswordView.vue'),
        },
        // Integrations — admin+
        {
          path: 'integrations',
          name: 'integrations',
          component: () => import('@/views/integration/IntegrationsView.vue'),
          meta: { roles: ['admin', 'super_admin'] },
        },
        {
          path: 'lab-results',
          name: 'lab-results',
          component: () => import('@/views/integration/LabResultsView.vue'),
        },
      ],
    },
    {
      path: '/:pathMatch(.*)*',
      redirect: '/dashboard',
    },
  ],
})

// ── Navigation guard ──────────────────────────────────────────────────────────
router.beforeEach(async (to) => {
  const auth = useAuthStore()

  if (to.name === 'login') {
    if (auth.isAuthenticated) return '/dashboard'
    return true
  }

  if (!auth.isAuthenticated) return '/login'

  // Load user if needed
  if (!auth.user) {
    try {
      await auth.fetchMe()
    } catch {
      return '/login'
    }
  }

  // Role check
  if (to.meta.roles?.length && auth.user) {
    if (!to.meta.roles.includes(auth.user.role)) {
      return '/dashboard'
    }
  }

  return true
})

export default router
