import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi } from '@/api/auth'
import type { User, Role } from '@/types/auth'
import { ADMIN_ROLES } from '@/utils/permissions'

export const useAuthStore = defineStore('auth', () => {
  // Access token lives only in memory (security); refresh token in localStorage
  const accessToken = ref<string>(localStorage.getItem('access_token') ?? '')
  const refreshToken = ref<string>(localStorage.getItem('refresh_token') ?? '')
  const user = ref<User | null>(null)

  const isAuthenticated = computed(() => !!accessToken.value)
  const isAdmin = computed(() => ADMIN_ROLES.includes(user.value?.role as Role))
  const isSuperAdmin = computed(() => user.value?.role === 'super_admin')
  const fullName = computed(() =>
    user.value ? `${user.value.first_name} ${user.value.last_name}` : '',
  )

  function hasRole(roles: Role[]): boolean {
    return !!user.value && roles.includes(user.value.role)
  }

  async function login(email: string, password: string): Promise<void> {
    const { data } = await authApi.login({ email, password })
    accessToken.value = data.access_token
    refreshToken.value = data.refresh_token
    localStorage.setItem('access_token', data.access_token)
    localStorage.setItem('refresh_token', data.refresh_token)
    await fetchMe()
  }

  async function logout(): Promise<void> {
    try {
      if (refreshToken.value) {
        await authApi.logout(refreshToken.value)
      }
    } finally {
      accessToken.value = ''
      refreshToken.value = ''
      user.value = null
      localStorage.removeItem('access_token')
      localStorage.removeItem('refresh_token')
    }
  }

  async function refreshTokens(): Promise<void> {
    const { data } = await authApi.refresh(refreshToken.value)
    accessToken.value = data.access_token
    refreshToken.value = data.refresh_token
    localStorage.setItem('access_token', data.access_token)
    localStorage.setItem('refresh_token', data.refresh_token)
  }

  async function fetchMe(): Promise<void> {
    const { data } = await authApi.me()
    user.value = data
  }

  return {
    accessToken,
    refreshToken,
    user,
    isAuthenticated,
    isAdmin,
    isSuperAdmin,
    fullName,
    hasRole,
    login,
    logout,
    refreshTokens,
    fetchMe,
  }
})
