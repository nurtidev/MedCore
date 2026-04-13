export interface DoctorWorkload {
  doctor_id: string
  doctor_name: string
  period: string
  total_appointments: number
  completed_count: number
  no_show_count: number
  no_show_rate: number
  workload_percent: number
}

export interface RevenueByDay {
  date: string
  revenue: number
  count: number
}

export interface ClinicRevenue {
  clinic_id: string
  period: string
  total_revenue: number
  currency: string
  payment_count: number
  avg_check: number
  revenue_by_day: RevenueByDay[]
}

export interface ScheduleFill {
  period: string
  total_slots: number
  booked_slots: number
  fill_rate: number
}

export interface DashboardData {
  subscription?: {
    plan?: { name: string; tier: string; max_doctors: number; max_patients: number }
    status: string
    doctors_used?: number
    patients_used?: number
  }
  analytics?: {
    revenue?: ClinicRevenue
    workload?: DoctorWorkload[]
  }
  partial?: boolean
}
