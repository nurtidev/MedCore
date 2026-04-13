import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAuthStore } from '@/stores/auth'

// Mock the API module
vi.mock('@/api/auth', () => ({
  authApi: {
    login: vi.fn(),
    logout: vi.fn(),
    refresh: vi.fn(),
    me: vi.fn(),
  },
}))

import { authApi } from '@/api/auth'

describe('useAuthStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    vi.clearAllMocks()
  })

  it('starts unauthenticated', () => {
    const store = useAuthStore()
    // localStorage cleared above
    store.accessToken = ''
    expect(store.isAuthenticated).toBe(false)
    expect(store.user).toBeNull()
  })

  it('login stores tokens and fetches user', async () => {
    const fakeTokens = {
      access_token: 'access-abc',
      refresh_token: 'refresh-xyz',
      expires_at: '2099-01-01T00:00:00Z',
    }
    const fakeUser = {
      id: 'u1',
      clinic_id: 'c1',
      email: 'admin@clinic.kz',
      first_name: 'Айдар',
      last_name: 'Сейтов',
      phone: '',
      role: 'admin' as const,
      is_active: true,
      created_at: '2025-01-01T00:00:00Z',
    }

    vi.mocked(authApi.login).mockResolvedValue({ data: fakeTokens } as any)
    vi.mocked(authApi.me).mockResolvedValue({ data: fakeUser } as any)

    const store = useAuthStore()
    await store.login('admin@clinic.kz', 'secret123')

    expect(store.accessToken).toBe('access-abc')
    expect(store.refreshToken).toBe('refresh-xyz')
    expect(store.isAuthenticated).toBe(true)
    expect(store.user?.email).toBe('admin@clinic.kz')
    expect(store.fullName).toBe('Айдар Сейтов')
    expect(localStorage.getItem('access_token')).toBe('access-abc')
  })

  it('logout clears state', async () => {
    vi.mocked(authApi.logout).mockResolvedValue({ data: {} } as any)

    const store = useAuthStore()
    store.accessToken = 'token'
    store.refreshToken = 'refresh'
    store.user = { id: 'u1', clinic_id: 'c1', email: 'x@y.kz', first_name: 'A', last_name: 'B', phone: '', role: 'admin', is_active: true, created_at: '' }

    await store.logout()

    expect(store.isAuthenticated).toBe(false)
    expect(store.user).toBeNull()
    expect(localStorage.getItem('access_token')).toBeNull()
  })

  it('hasRole returns true for matching role', () => {
    const store = useAuthStore()
    store.user = { id: 'u1', clinic_id: 'c1', email: 'x@y.kz', first_name: 'A', last_name: 'B', phone: '', role: 'admin', is_active: true, created_at: '' }
    expect(store.hasRole(['admin', 'super_admin'])).toBe(true)
    expect(store.hasRole(['doctor'])).toBe(false)
  })
})
