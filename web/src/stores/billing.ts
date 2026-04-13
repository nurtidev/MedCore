import { defineStore } from 'pinia'
import { ref } from 'vue'
import { billingApi } from '@/api/billing'
import type { Invoice, Subscription, Plan } from '@/types/billing'

export const useBillingStore = defineStore('billing', () => {
  const invoices = ref<Invoice[]>([])
  const total = ref(0)
  const subscription = ref<Subscription | null>(null)
  const plans = ref<Plan[]>([])
  const loading = ref(false)

  async function fetchInvoices(params?: { status?: string; limit?: number; offset?: number }): Promise<void> {
    loading.value = true
    try {
      const { data } = await billingApi.listInvoices(params)
      invoices.value = data.invoices
      total.value = data.total
    } finally {
      loading.value = false
    }
  }

  async function fetchSubscription(): Promise<void> {
    const { data } = await billingApi.getCurrentSubscription()
    subscription.value = data
  }

  async function fetchPlans(): Promise<void> {
    const { data } = await billingApi.listPlans()
    plans.value = data
  }

  return { invoices, total, subscription, plans, loading, fetchInvoices, fetchSubscription, fetchPlans }
})
