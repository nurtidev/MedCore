<template>
  <div class="space-y-6">
    <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
      <div v-if="loading" v-for="i in 3" :key="i" class="bg-white rounded-xl border p-5 animate-pulse">
        <div class="h-4 bg-gray-200 rounded w-1/2 mb-3" /><div class="h-10 bg-gray-200 rounded" />
      </div>
      <template v-else>
        <StatCard title="Всего слотов" :value="String(schedule?.total_slots ?? 0)" />
        <StatCard title="Занято" :value="String(schedule?.booked_slots ?? 0)" />
        <StatCard title="Заполняемость" :value="formatPercent(schedule?.fill_rate ?? 0)" highlight />
      </template>
    </div>

    <div class="bg-white rounded-xl border p-5">
      <h2 class="text-sm font-semibold text-gray-700 mb-6">Воронка пациентов</h2>
      <FunnelChart :data="funnelData" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, defineComponent, h } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { analyticsApi } from '@/api/analytics'
import { formatPercent } from '@/utils/format'
import FunnelChart from '@/components/charts/FunnelChart.vue'
import type { ScheduleFill } from '@/types/analytics'

const auth = useAuthStore()
const loading = ref(false)
const schedule = ref<ScheduleFill | null>(null)

const StatCard = defineComponent({
  props: ['title', 'value', 'highlight'],
  setup: (p) => () => h('div', { class: 'bg-white rounded-xl border p-5' }, [
    h('p', { class: 'text-sm text-gray-500' }, p.title),
    h('p', { class: `text-3xl font-bold mt-1 ${p.highlight ? 'text-primary-600' : 'text-gray-900'}` }, p.value),
  ]),
})

const funnelData = computed(() => [
  { name: 'Новый пациент', value: 100 },
  { name: 'Первый визит', value: 75 },
  { name: 'Повторный', value: 50 },
  { name: 'Постоянный', value: 30 },
])

onMounted(async () => {
  loading.value = true
  try {
    const { data } = await analyticsApi.scheduleFill({ clinic_id: auth.user!.clinic_id, period: 'month' })
    schedule.value = data
  } finally {
    loading.value = false
  }
})
</script>
