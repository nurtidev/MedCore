<template>
  <div class="space-y-6">
    <div class="bg-white rounded-xl border p-5">
      <h2 class="text-sm font-semibold text-gray-700 mb-4">Загруженность врачей</h2>
      <div v-if="loading" class="h-80 animate-pulse bg-gray-100 rounded" />
      <WorkloadChart v-else-if="store.workload.length" :data="store.workload" />
      <p v-else class="text-center py-20 text-gray-400">{{ $t('common.no_data') }}</p>
    </div>

    <!-- Table -->
    <div class="bg-white rounded-xl border overflow-hidden">
      <table class="min-w-full divide-y divide-gray-200">
        <thead class="bg-gray-50">
          <tr>
            <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Врач</th>
            <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Приёмы</th>
            <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Выполнено</th>
            <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">No-show</th>
            <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Загруженность</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-200">
          <tr v-for="d in store.workload" :key="d.doctor_id" class="hover:bg-gray-50">
            <td class="px-4 py-3 text-sm font-medium text-gray-900">{{ d.doctor_name }}</td>
            <td class="px-4 py-3 text-sm text-gray-600">{{ d.total_appointments }}</td>
            <td class="px-4 py-3 text-sm text-gray-600">{{ d.completed_count }}</td>
            <td class="px-4 py-3 text-sm text-gray-600">{{ formatPercent(d.no_show_rate) }}</td>
            <td class="px-4 py-3">
              <div class="flex items-center gap-2">
                <div class="flex-1 h-2 bg-gray-100 rounded-full">
                  <div :class="['h-full rounded-full', workloadColor(d.workload_percent)]"
                       :style="`width:${d.workload_percent}%`" />
                </div>
                <span class="text-xs font-medium w-10">{{ formatPercent(d.workload_percent) }}</span>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useAnalyticsStore } from '@/stores/analytics'
import { formatPercent } from '@/utils/format'
import WorkloadChart from '@/components/charts/WorkloadChart.vue'

const auth = useAuthStore()
const store = useAnalyticsStore()
const loading = ref(false)

function workloadColor(pct: number) {
  if (pct >= 80) return 'bg-green-500'
  if (pct >= 50) return 'bg-yellow-400'
  return 'bg-red-500'
}

onMounted(async () => {
  loading.value = true
  try {
    await store.fetchWorkload(auth.user!.clinic_id, 'month')
  } finally {
    loading.value = false
  }
})
</script>
