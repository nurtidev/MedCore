export type Role = 'doctor' | 'coordinator' | 'admin' | 'super_admin'

export interface User {
  id: string
  clinic_id: string
  email: string
  first_name: string
  last_name: string
  phone: string
  role: Role
  is_active: boolean
  created_at: string
}

export interface TokenPair {
  access_token: string
  refresh_token: string
  expires_at: string
}

export interface LoginRequest {
  email: string
  password: string
}

export interface RegisterRequest {
  clinic_id: string
  email: string
  password: string
  first_name: string
  last_name: string
  iin?: string
  phone?: string
  role: Role
}

export interface UpdateUserRequest {
  first_name?: string
  last_name?: string
  phone?: string
}
