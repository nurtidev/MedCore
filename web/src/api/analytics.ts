import apiClient from './client'
import type { ClinicRevenue, DoctorWorkload, ScheduleFill, DashboardData } from '@/types/analytics'

export const analyticsApi = {
  dashboard: (params?: { clinic_id?: string; period?: string }) =>
    apiClient.get<DashboardData>('/api/v1/dashboard', { params }),

  revenue: (params: { clinic_id: string; period: string }) =>
    apiClient.get<ClinicRevenue>('/api/v1/analytics/revenue', { params }),

  doctorWorkload: (params: { clinic_id: string; period: string }) =>
    apiClient.get<DoctorWorkload[]>('/api/v1/analytics/doctors/workload', { params }),

  scheduleFill: (params: { clinic_id: string; period: string }) =>
    apiClient.get<ScheduleFill>('/api/v1/analytics/schedule/fill-rate', { params }),

  exportExcel: (params: { clinic_id: string; period: string }) =>
    apiClient.get('/api/v1/analytics/export/excel', { params, responseType: 'blob' }),
}
