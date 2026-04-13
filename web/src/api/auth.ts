import apiClient from './client'
import type { LoginRequest, TokenPair, User, RegisterRequest, UpdateUserRequest } from '@/types/auth'

export const authApi = {
  login: (data: LoginRequest) =>
    apiClient.post<TokenPair>('/api/v1/auth/login', data),

  register: (data: RegisterRequest) =>
    apiClient.post<User>('/api/v1/auth/register', data),

  refresh: (refreshToken: string) =>
    apiClient.post<TokenPair>('/api/v1/auth/refresh', { refresh_token: refreshToken }),

  logout: (refreshToken: string) =>
    apiClient.post('/api/v1/auth/logout', { refresh_token: refreshToken }),

  me: () =>
    apiClient.get<User>('/api/v1/auth/me'),

  updateMe: (data: UpdateUserRequest) =>
    apiClient.put<User>('/api/v1/auth/me', data),

  changePassword: (oldPassword: string, newPassword: string) =>
    apiClient.post('/api/v1/auth/change-password', { old_password: oldPassword, new_password: newPassword }),

  listUsers: (params?: { limit?: number; offset?: number; clinic_id?: string }) =>
    apiClient.get<{ users: User[]; total: number }>('/api/v1/users', { params }),

  getUser: (id: string) =>
    apiClient.get<User>(`/api/v1/users/${id}`),

  updateUser: (id: string, data: UpdateUserRequest) =>
    apiClient.put<User>(`/api/v1/users/${id}`, data),

  deactivateUser: (id: string) =>
    apiClient.delete(`/api/v1/users/${id}`),
}
