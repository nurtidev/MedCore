<template>
  <div class="bg-white rounded-lg border border-gray-200 p-4 flex items-center justify-between hover:shadow-sm transition">
    <div>
      <p class="text-sm font-medium text-gray-900">{{ invoice.service_name }}</p>
      <p class="text-xs text-gray-500 mt-0.5">{{ formatDate(invoice.due_at) }}</p>
    </div>
    <div class="flex items-center gap-3">
      <PaymentStatusBadge :status="invoice.status" />
      <span class="text-sm font-semibold text-gray-900">{{ formatKZT(invoice.amount) }}</span>
      <button class="text-primary-600 hover:text-primary-800 text-xs font-medium" @click="$emit('download', invoice.id)">
        PDF
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import PaymentStatusBadge from './PaymentStatusBadge.vue'
import { formatDate, formatKZT } from '@/utils/format'
import type { Invoice } from '@/types/billing'

defineProps<{ invoice: Invoice }>()
defineEmits<{ download: [id: string] }>()
</script>
