import apiClient from './client'
import type { Invoice, Plan, Subscription, PaymentLink } from '@/types/billing'

export const billingApi = {
  // Invoices
  listInvoices: (params?: { status?: string; limit?: number; offset?: number }) =>
    apiClient.get<{ invoices: Invoice[]; total: number }>('/api/v1/invoices', { params }),

  getInvoice: (id: string) =>
    apiClient.get<Invoice>(`/api/v1/invoices/${id}`),

  downloadInvoicePDF: (id: string) =>
    apiClient.get(`/api/v1/invoices/${id}/pdf`, { responseType: 'blob' }),

  // Payments
  createPaymentLink: (data: {
    invoice_id: string
    provider: string
    return_url: string
  }) => apiClient.post<PaymentLink>('/api/v1/payments', data),

  // Plans
  listPlans: () =>
    apiClient.get<Plan[]>('/api/v1/plans'),

  // Subscriptions
  getCurrentSubscription: () =>
    apiClient.get<Subscription>('/api/v1/subscriptions/current'),

  cancelSubscription: (id: string) =>
    apiClient.delete(`/api/v1/subscriptions/${id}`),

  upgradeSubscription: (planId: string) =>
    apiClient.post<Subscription>('/api/v1/subscriptions/upgrade', { plan_id: planId }),
}
