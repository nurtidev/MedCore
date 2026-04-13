<template>
  <div class="space-y-4">
    <!-- Filters -->
    <div class="bg-white rounded-xl border p-4 flex flex-wrap gap-3 items-center">
      <BaseInput
        v-model="patientId"
        placeholder="ID пациента…"
        class="w-56"
        @input="debouncedLoad"
      />
      <span class="ml-auto text-sm text-gray-500">Всего: {{ total }}</span>
    </div>

    <!-- Table -->
    <div class="bg-white rounded-xl border overflow-hidden">
      <div v-if="loading" class="p-8 flex justify-center"><LoadingSpinner /></div>
      <template v-else>
        <table class="min-w-full divide-y divide-gray-200">
          <thead class="bg-gray-50">
            <tr>
              <th v-for="col in cols" :key="col"
                class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                {{ col }}
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200">
            <tr v-if="!results.length">
              <td colspan="7" class="px-4 py-8 text-center text-sm text-gray-400">
                {{ $t('common.no_data') }}
              </td>
            </tr>
            <tr v-for="r in results" :key="r.id" class="hover:bg-gray-50">
              <td class="px-4 py-3 text-xs font-mono text-gray-400">{{ r.id.slice(0, 8) }}…</td>
              <td class="px-4 py-3 text-sm text-gray-900">{{ r.test_name }}</td>
              <td class="px-4 py-3 text-sm font-semibold text-gray-900">
                {{ r.result_value }} <span class="text-xs font-normal text-gray-500">{{ r.unit }}</span>
              </td>
              <td class="px-4 py-3 text-sm text-gray-500">{{ r.reference_range }}</td>
              <td class="px-4 py-3">
                <BaseBadge :variant="statusVariant(r.status)">{{ statusLabel(r.status) }}</BaseBadge>
              </td>
              <td class="px-4 py-3 text-sm text-gray-500 uppercase">{{ r.source }}</td>
              <td class="px-4 py-3 text-sm text-gray-500">{{ formatDate(r.collected_at) }}</td>
            </tr>
          </tbody>
        </table>

        <!-- Pagination -->
        <div v-if="pagination.totalPages.value > 1" class="flex justify-between items-center px-4 py-3 border-t">
          <span class="text-sm text-gray-500">{{ pagination.page.value }} / {{ pagination.totalPages.value }}</span>
          <div class="flex gap-1">
            <button :disabled="!pagination.hasPrev.value"
              class="px-3 py-1 border rounded text-sm disabled:opacity-40"
              @click="changePage(pagination.page.value - 1)">←</button>
            <button :disabled="!pagination.hasNext.value"
              class="px-3 py-1 border rounded text-sm disabled:opacity-40"
              @click="changePage(pagination.page.value + 1)">→</button>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { integrationApi } from '@/api/integration'
import type { LabResult } from '@/api/integration'
import { usePagination } from '@/composables/usePagination'
import { formatDate } from '@/utils/format'
import BaseInput from '@/components/ui/BaseInput.vue'
import BaseBadge from '@/components/ui/BaseBadge.vue'
import LoadingSpinner from '@/components/ui/LoadingSpinner.vue'

const loading = ref(false)
const results = ref<LabResult[]>([])
const total = ref(0)
const patientId = ref('')
const pagination = usePagination(25)

let debounceTimer: ReturnType<typeof setTimeout>

const cols = ['ID', 'Тест', 'Результат', 'Норма', 'Статус', 'Источник', 'Дата забора']

function statusVariant(status: LabResult['status']) {
  return status === 'normal' ? 'green' : status === 'abnormal' ? 'yellow' : 'red'
}

function statusLabel(status: LabResult['status']) {
  return status === 'normal' ? 'Норма' : status === 'abnormal' ? 'Отклонение' : 'Критично'
}

function debouncedLoad() {
  clearTimeout(debounceTimer)
  debounceTimer = setTimeout(load, 300)
}

async function load() {
  loading.value = true
  try {
    const { data } = await integrationApi.listLabResults({
      patient_id: patientId.value || undefined,
      limit: pagination.pageSize,
      offset: pagination.offset.value,
    })
    results.value = data.results ?? []
    total.value = data.total ?? 0
    pagination.total.value = total.value
  } finally {
    loading.value = false
  }
}

async function changePage(p: number) {
  pagination.goTo(p)
  await load()
}

onMounted(load)
</script>
