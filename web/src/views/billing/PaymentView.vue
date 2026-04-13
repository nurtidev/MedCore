<template>
  <div class="max-w-md mx-auto">
    <div class="bg-white rounded-xl border p-6">
      <h2 class="text-lg font-semibold text-gray-900 mb-6">Оплата счёта</h2>

      <p v-if="loadingInvoice" class="text-gray-400 text-sm">Загрузка…</p>
      <template v-else-if="invoice">
        <div class="bg-gray-50 rounded-lg p-4 mb-6">
          <p class="text-sm text-gray-600">{{ invoice.service_name }}</p>
          <p class="text-2xl font-bold text-gray-900 mt-1">{{ formatKZT(invoice.amount) }}</p>
        </div>

        <p class="text-sm font-medium text-gray-700 mb-3">Способ оплаты</p>
        <div class="space-y-3 mb-6">
          <label
            v-for="p in providers"
            :key="p.value"
            :class="['flex items-center gap-3 border rounded-lg p-3 cursor-pointer transition',
              provider === p.value ? 'border-primary-500 bg-primary-50' : 'border-gray-200 hover:border-gray-300']"
          >
            <input type="radio" :value="p.value" v-model="provider" class="text-primary-600" />
            <span class="text-2xl">{{ p.icon }}</span>
            <span class="text-sm font-medium">{{ p.label }}</span>
          </label>
        </div>

        <p v-if="error" class="text-sm text-red-600 mb-4">{{ error }}</p>

        <BaseButton class="w-full" :loading="loading" @click="pay">
          Перейти к оплате
        </BaseButton>
        <RouterLink :to="`/billing/invoices/${route.params.invoiceId}`">
          <BaseButton variant="ghost" class="w-full mt-2">Отмена</BaseButton>
        </RouterLink>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { billingApi } from '@/api/billing'
import { formatKZT } from '@/utils/format'
import BaseButton from '@/components/ui/BaseButton.vue'
import type { Invoice } from '@/types/billing'

const route = useRoute()
const invoice = ref<Invoice | null>(null)
const loadingInvoice = ref(true)
const loading = ref(false)
const error = ref('')
const provider = ref('kaspi')

const providers = [
  { value: 'kaspi', label: 'Kaspi Pay', icon: '🇰🇿' },
  { value: 'stripe', label: 'Visa / Mastercard', icon: '💳' },
]

onMounted(async () => {
  try {
    const { data } = await billingApi.getInvoice(route.params.invoiceId as string)
    invoice.value = data
  } finally {
    loadingInvoice.value = false
  }
})

async function pay() {
  if (!invoice.value) return
  loading.value = true
  error.value = ''
  try {
    const { data } = await billingApi.createPaymentLink({
      invoice_id: invoice.value.id,
      provider: provider.value,
      return_url: `${window.location.origin}/billing/invoices/${invoice.value.id}`,
    })
    window.location.href = data.url
  } catch {
    error.value = 'Не удалось создать ссылку для оплаты. Попробуйте позже.'
  } finally {
    loading.value = false
  }
}
</script>
