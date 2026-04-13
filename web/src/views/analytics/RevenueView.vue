<template>
  <div class="space-y-6">
    <!-- Controls -->
    <div class="flex items-center justify-between">
      <div class="flex gap-2">
        <button
          v-for="p in periods"
          :key="p.value"
          :class="['px-4 py-2 rounded-lg text-sm font-medium transition',
            period === p.value ? 'bg-primary-600 text-white' : 'bg-white border text-gray-600 hover:bg-gray-50']"
          @click="changePeriod(p.value)"
        >{{ p.label }}</button>
      </div>
      <BaseButton variant="secondary" size="sm" @click="exportData">
        ⬇ Excel
      </BaseButton>
    </div>

    <!-- Chart -->
    <div class="bg-white rounded-xl border p-5">
      <h2 class="text-sm font-semibold text-gray-700 mb-4">Выручка по дням</h2>
      <div v-if="loading" class="h-80 animate-pulse bg-gray-100 rounded" />
      <RevenueChart v-else-if="revenue?.revenue_by_day?.length" :data="revenue.revenue_by_day" />
      <p v-else class="text-center py-20 text-gray-400">{{ $t('common.no_data') }}</p>
    </div>

    <!-- Summary cards -->
    <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
      <StatCard title="Общая выручка" :value="formatKZT(revenue?.total_revenue ?? 0)" />
      <StatCard title="Кол-во платежей" :value="String(revenue?.payment_count ?? 0)" />
      <StatCard title="Средний чек" :value="formatKZT(revenue?.avg_check ?? 0)" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, defineComponent, h } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useAnalyticsStore } from '@/stores/analytics'
import { useExport } from '@/composables/useExport'
import { formatKZT } from '@/utils/format'
import RevenueChart from '@/components/charts/RevenueChart.vue'
import BaseButton from '@/components/ui/BaseButton.vue'

const auth = useAuthStore()
const store = useAnalyticsStore()
const { downloadAnalyticsExcel } = useExport()

const period = ref('month')
const loading = ref(false)
const revenue = computed(() => store.revenue)

const periods = [
  { value: 'month', label: 'Месяц' },
  { value: 'quarter', label: 'Квартал' },
  { value: 'year', label: 'Год' },
]

const StatCard = defineComponent({
  props: ['title', 'value'],
  setup: (p) => () => h('div', { class: 'bg-white rounded-xl border p-5' }, [
    h('p', { class: 'text-sm text-gray-500' }, p.title),
    h('p', { class: 'text-2xl font-bold text-gray-900 mt-1' }, p.value),
  ]),
})

async function load() {
  loading.value = true
  try {
    await store.fetchRevenue(auth.user!.clinic_id, period.value)
  } finally {
    loading.value = false
  }
}

async function changePeriod(p: string) {
  period.value = p
  await load()
}

async function exportData() {
  await downloadAnalyticsExcel(auth.user!.clinic_id, period.value)
}

onMounted(load)
</script>
