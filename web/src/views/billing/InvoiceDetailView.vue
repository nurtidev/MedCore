<template>
  <div>
    <div v-if="loading" class="animate-pulse space-y-4">
      <div class="h-8 bg-gray-200 rounded w-1/3" />
      <div class="h-48 bg-gray-200 rounded" />
    </div>

    <template v-else-if="invoice">
      <div class="bg-white rounded-xl border p-6 max-w-2xl">
        <div class="flex items-start justify-between mb-6">
          <div>
            <h2 class="text-lg font-semibold text-gray-900">{{ invoice.service_name }}</h2>
            <p class="text-sm text-gray-500 mt-0.5 font-mono">{{ invoice.id }}</p>
          </div>
          <PaymentStatusBadge :status="invoice.status" />
        </div>

        <dl class="grid grid-cols-2 gap-4 text-sm">
          <div><dt class="text-gray-500">Сумма</dt><dd class="font-bold text-xl text-gray-900 mt-0.5">{{ formatKZT(invoice.amount) }}</dd></div>
          <div><dt class="text-gray-500">Валюта</dt><dd class="font-medium mt-0.5">{{ invoice.currency }}</dd></div>
          <div><dt class="text-gray-500">Срок оплаты</dt><dd class="mt-0.5">{{ formatDate(invoice.due_at) }}</dd></div>
          <div><dt class="text-gray-500">Оплачено</dt><dd class="mt-0.5">{{ invoice.paid_at ? formatDate(invoice.paid_at) : '—' }}</dd></div>
          <div><dt class="text-gray-500">Создан</dt><dd class="mt-0.5">{{ formatDate(invoice.created_at) }}</dd></div>
        </dl>

        <div class="flex gap-3 mt-8">
          <BaseButton @click="exportHook.downloadInvoicePDF(invoice.id)">⬇ Скачать PDF</BaseButton>
          <RouterLink
            v-if="invoice.status !== 'paid' && invoice.status !== 'voided'"
            :to="`/billing/payment/${invoice.id}`"
          >
            <BaseButton variant="secondary">Оплатить</BaseButton>
          </RouterLink>
          <RouterLink to="/billing/invoices">
            <BaseButton variant="ghost">← Назад</BaseButton>
          </RouterLink>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { billingApi } from '@/api/billing'
import { useExport } from '@/composables/useExport'
import { formatKZT, formatDate } from '@/utils/format'
import PaymentStatusBadge from '@/components/billing/PaymentStatusBadge.vue'
import BaseButton from '@/components/ui/BaseButton.vue'
import type { Invoice } from '@/types/billing'

const route = useRoute()
const exportHook = useExport()
const loading = ref(true)
const invoice = ref<Invoice | null>(null)

onMounted(async () => {
  try {
    const { data } = await billingApi.getInvoice(route.params.id as string)
    invoice.value = data
  } finally {
    loading.value = false
  }
})
</script>
