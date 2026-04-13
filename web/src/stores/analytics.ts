import { defineStore } from 'pinia'
import { ref } from 'vue'
import { analyticsApi } from '@/api/analytics'
import type { DashboardData, ClinicRevenue, DoctorWorkload } from '@/types/analytics'

export const useAnalyticsStore = defineStore('analytics', () => {
  const dashboard = ref<DashboardData | null>(null)
  const revenue = ref<ClinicRevenue | null>(null)
  const workload = ref<DoctorWorkload[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchDashboard(clinicId?: string, period?: string): Promise<void> {
    loading.value = true
    error.value = null
    try {
      const { data } = await analyticsApi.dashboard({ clinic_id: clinicId, period })
      dashboard.value = data
    } catch (e: unknown) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  async function fetchRevenue(clinicId: string, period: string): Promise<void> {
    const { data } = await analyticsApi.revenue({ clinic_id: clinicId, period })
    revenue.value = data
  }

  async function fetchWorkload(clinicId: string, period: string): Promise<void> {
    const { data } = await analyticsApi.doctorWorkload({ clinic_id: clinicId, period })
    workload.value = data
  }

  return { dashboard, revenue, workload, loading, error, fetchDashboard, fetchRevenue, fetchWorkload }
})
