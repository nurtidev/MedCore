export type InvoiceStatus = 'draft' | 'sent' | 'paid' | 'overdue' | 'voided'
export type PaymentStatus = 'pending' | 'processing' | 'completed' | 'failed' | 'refunded'
export type PaymentProvider = 'kaspi' | 'stripe'
export type PlanTier = 'basic' | 'pro' | 'enterprise'

export interface Invoice {
  id: string
  clinic_id: string
  patient_id: string
  service_name: string
  amount: string
  currency: string
  status: InvoiceStatus
  due_at: string
  paid_at?: string
  created_at: string
}

export interface Payment {
  id: string
  invoice_id: string
  clinic_id: string
  patient_id: string
  amount: string
  currency: string
  status: PaymentStatus
  provider: PaymentProvider
  created_at: string
}

export interface Plan {
  id: string
  tier: PlanTier
  name: string
  price_monthly: string
  currency: string
  max_doctors: number
  max_patients: number
  features: string[]
}

export interface Subscription {
  id: string
  clinic_id: string
  plan_id: string
  plan?: Plan
  status: 'active' | 'cancelled' | 'expired' | 'trial'
  current_period_start: string
  current_period_end: string
  doctors_used?: number
  patients_used?: number
}

export interface PaymentLink {
  payment_id: string
  url: string
  expires_at: string
}
