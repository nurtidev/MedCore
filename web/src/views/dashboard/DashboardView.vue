<template>
  <div>
    <div v-if="store.loading" class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-4 mb-6">
      <div v-for="i in 4" :key="i" class="bg-white rounded-xl border p-5 animate-pulse">
        <div class="h-4 bg-gray-200 rounded w-1/2 mb-3" />
        <div class="h-8 bg-gray-200 rounded w-3/4" />
      </div>
    </div>

    <template v-else>
      <!-- KPI Cards -->
      <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-4 mb-6">
        <KpiCard
          :title="$t('dashboard.revenue_month')"
          :value="formatKZT(analytics?.revenue?.total_revenue ?? 0)"
          icon="💰"
          trend="+12%"
          trend-up
        />
        <KpiCard
          :title="$t('dashboard.patients_count')"
          :value="String(analytics?.revenue?.payment_count ?? 0)"
          icon="👥"
        />
        <KpiCard
          :title="$t('dashboard.fill_rate')"
          :value="formatPercent(fillRate)"
          icon="📅"
        />
        <KpiCard
          :title="$t('dashboard.no_show_rate')"
          :value="formatPercent(noShowRate)"
          icon="⚠️"
          :trend-up="false"
        />
      </div>

      <div class="grid grid-cols-1 xl:grid-cols-3 gap-6">
        <!-- Revenue chart -->
        <div class="xl:col-span-2 bg-white rounded-xl border p-5">
          <h2 class="text-sm font-semibold text-gray-700 mb-4">Выручка по дням</h2>
          <RevenueChart v-if="revenueByDay.length" :data="revenueByDay" />
          <p v-else class="text-sm text-gray-400 text-center py-12">{{ $t('common.no_data') }}</p>
        </div>

        <!-- Current plan -->
        <div class="bg-white rounded-xl border p-5">
          <h2 class="text-sm font-semibold text-gray-700 mb-4">{{ $t('dashboard.current_plan') }}</h2>
          <template v-if="sub">
            <p class="text-lg font-bold text-primary-600">{{ sub.plan?.name ?? sub.plan?.tier }}</p>
            <BaseBadge :variant="sub.status === 'active' ? 'green' : 'yellow'" class="mt-1">
              {{ sub.status }}
            </BaseBadge>
            <div class="mt-4 space-y-3">
              <ProgressBar
                :label="$t('dashboard.doctors_used')"
                :value="sub.doctors_used ?? 0"
                :max="sub.plan?.max_doctors ?? 1"
              />
            </div>
          </template>
          <p v-else class="text-sm text-gray-400">Нет активной подписки</p>
        </div>

        <!-- Recent invoices -->
        <div class="xl:col-span-3 bg-white rounded-xl border p-5">
          <div class="flex items-center justify-between mb-4">
            <h2 class="text-sm font-semibold text-gray-700">{{ $t('dashboard.recent_invoices') }}</h2>
            <RouterLink to="/billing/invoices" class="text-xs text-primary-600 hover:underline">Все счета →</RouterLink>
          </div>
          <div class="space-y-2">
            <InvoiceCard
              v-for="inv in recentInvoices"
              :key="inv.id"
              :invoice="inv"
              @download="exportHook.downloadInvoicePDF"
            />
            <p v-if="!recentInvoices.length" class="text-sm text-gray-400 text-center py-4">{{ $t('common.no_data') }}</p>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, defineComponent, h } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useAnalyticsStore } from '@/stores/analytics'
import { useBillingStore } from '@/stores/billing'
import { useExport } from '@/composables/useExport'
import { formatKZT, formatPercent } from '@/utils/format'
import RevenueChart from '@/components/charts/RevenueChart.vue'
import InvoiceCard from '@/components/billing/InvoiceCard.vue'
import BaseBadge from '@/components/ui/BaseBadge.vue'

const auth = useAuthStore()
const store = useAnalyticsStore()
const billing = useBillingStore()
const exportHook = useExport()

// ── Inline sub-components ────────────────────────────────────────────────────
const KpiCard = defineComponent({
  props: ['title', 'value', 'icon', 'trend', 'trendUp'],
  setup(props) {
    return () => h('div', { class: 'bg-white rounded-xl border p-5' }, [
      h('div', { class: 'flex items-center justify-between mb-2' }, [
        h('span', { class: 'text-2xl' }, props.icon),
        props.trend && h('span', {
          class: `text-xs font-medium ${props.trendUp ? 'text-green-600' : 'text-red-500'}`
        }, props.trend),
      ]),
      h('p', { class: 'text-2xl font-bold text-gray-900' }, props.value),
      h('p', { class: 'text-sm text-gray-500 mt-0.5' }, props.title),
    ])
  },
})

const ProgressBar = defineComponent({
  props: ['label', 'value', 'max'],
  setup(props) {
    const pct = computed(() => props.max ? Math.min(100, Math.round(props.value / props.max * 100)) : 0)
    return () => h('div', [
      h('div', { class: 'flex justify-between text-xs text-gray-500 mb-1' }, [
        h('span', props.label),
        h('span', `${props.value} / ${props.max}`),
      ]),
      h('div', { class: 'h-2 bg-gray-100 rounded-full overflow-hidden' }, [
        h('div', { class: 'h-full bg-primary-500 rounded-full transition-all', style: `width:${pct.value}%` }),
      ]),
    ])
  },
})

const analytics = computed(() => store.dashboard?.analytics)
const sub = computed(() => store.dashboard?.subscription)
const revenueByDay = computed(() => analytics.value?.revenue?.revenue_by_day ?? [])
const fillRate = computed(() => 0)
const noShowRate = computed(() => {
  const w = analytics.value?.workload
  if (!w?.length) return 0
  return w.reduce((s, d) => s + d.no_show_rate, 0) / w.length
})
const recentInvoices = computed(() => billing.invoices.slice(0, 5))

onMounted(async () => {
  const clinicId = auth.user?.clinic_id
  await Promise.all([
    store.fetchDashboard(clinicId, 'month'),
    billing.fetchInvoices({ limit: 5 }),
  ])
})
</script>
