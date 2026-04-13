<template>
  <BaseBadge :variant="color">{{ label }}</BaseBadge>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseBadge from '@/components/ui/BaseBadge.vue'
import type { InvoiceStatus } from '@/types/billing'

const props = defineProps<{ status: InvoiceStatus }>()
const { t } = useI18n()

const color = computed(() => ({
  draft: 'gray',
  sent: 'blue',
  paid: 'green',
  overdue: 'red',
  voided: 'gray',
} as const)[props.status] ?? 'gray')

const label = computed(() => t(`invoice_status.${props.status}`))
</script>
