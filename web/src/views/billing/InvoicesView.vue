<template>
  <div class="space-y-4">
    <!-- Filters -->
    <div class="bg-white rounded-xl border p-4 flex flex-wrap gap-3 items-center">
      <select
        v-model="statusFilter"
        class="border border-gray-300 rounded-lg px-3 py-2 text-sm"
        @change="load"
      >
        <option value="">Все статусы</option>
        <option v-for="s in statuses" :key="s.value" :value="s.value">{{ s.label }}</option>
      </select>
      <span class="ml-auto text-sm text-gray-500">{{ $t('common.total') }}: {{ billing.total }}</span>
    </div>

    <!-- Table -->
    <div class="bg-white rounded-xl border overflow-hidden">
      <div v-if="billing.loading" class="p-8 flex justify-center">
        <LoadingSpinner />
      </div>
      <template v-else>
        <table class="min-w-full divide-y divide-gray-200">
          <thead class="bg-gray-50">
            <tr>
              <th v-for="col in cols" :key="col" class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                {{ col }}
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200">
            <tr v-if="!billing.invoices.length">
              <td colspan="6" class="px-4 py-8 text-center text-sm text-gray-400">{{ $t('common.no_data') }}</td>
            </tr>
            <tr v-for="inv in billing.invoices" :key="inv.id" class="hover:bg-gray-50">
              <td class="px-4 py-3 text-sm font-mono text-gray-500">{{ inv.id.slice(0, 8) }}…</td>
              <td class="px-4 py-3 text-sm text-gray-900">{{ inv.service_name }}</td>
              <td class="px-4 py-3 text-sm font-semibold text-gray-900">{{ formatKZT(inv.amount) }}</td>
              <td class="px-4 py-3"><PaymentStatusBadge :status="inv.status" /></td>
              <td class="px-4 py-3 text-sm text-gray-500">{{ formatDate(inv.due_at) }}</td>
              <td class="px-4 py-3">
                <div class="flex gap-2">
                  <RouterLink :to="`/billing/invoices/${inv.id}`"
                    class="text-xs text-primary-600 hover:underline">Детали</RouterLink>
                  <button class="text-xs text-gray-500 hover:text-gray-700"
                    @click="exportHook.downloadInvoicePDF(inv.id)">PDF</button>
                </div>
              </td>
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
import { useBillingStore } from '@/stores/billing'
import { useExport } from '@/composables/useExport'
import { usePagination } from '@/composables/usePagination'
import { formatKZT, formatDate } from '@/utils/format'
import PaymentStatusBadge from '@/components/billing/PaymentStatusBadge.vue'
import LoadingSpinner from '@/components/ui/LoadingSpinner.vue'

const billing = useBillingStore()
const exportHook = useExport()
const pagination = usePagination(20)
const statusFilter = ref('')
const cols = ['ID', 'Услуга', 'Сумма', 'Статус', 'Срок', '']

const statuses = [
  { value: 'draft', label: 'Черновик' },
  { value: 'sent', label: 'Отправлен' },
  { value: 'paid', label: 'Оплачен' },
  { value: 'overdue', label: 'Просрочен' },
]

async function load() {
  await billing.fetchInvoices({
    status: statusFilter.value || undefined,
    limit: pagination.pageSize,
    offset: pagination.offset.value,
  })
  pagination.total.value = billing.total
}

async function changePage(p: number) {
  pagination.goTo(p)
  await load()
}

onMounted(load)
</script>
