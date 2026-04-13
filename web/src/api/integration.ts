import apiClient from './client'

export interface LabResult {
  id: string
  patient_id: string
  source: string
  test_name: string
  result_value: string
  unit: string
  reference_range: string
  status: 'normal' | 'abnormal' | 'critical'
  collected_at: string
  received_at: string
}

export interface IntegrationConfig {
  id: string
  clinic_id: string
  integration_type: string
  is_enabled: boolean
  config: Record<string, string>
  created_at: string
}

export const integrationApi = {
  listLabResults: (params?: { patient_id?: string; limit?: number; offset?: number }) =>
    apiClient.get<{ results: LabResult[]; total: number }>('/api/v1/lab-results', { params }),

  listIntegrations: () =>
    apiClient.get<IntegrationConfig[]>('/api/v1/integrations'),

  updateIntegration: (id: string, data: Partial<IntegrationConfig>) =>
    apiClient.put<IntegrationConfig>(`/api/v1/integrations/${id}`, data),

  syncPatient: (patientId: string) =>
    apiClient.post(`/api/v1/sync/patient/${patientId}`),
}
