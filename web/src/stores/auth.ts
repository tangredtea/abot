import { create } from 'zustand'
import { apiFetch } from '@/lib/api'
import { setToken, clearToken, getToken } from '@/lib/auth'

interface User {
  id: string
  email: string
  display_name: string
  role: string
  tenants: string[]
}

interface AuthState {
  token: string | null
  user: User | null
  loading: boolean
  error: string | null
  login: (email: string, password: string) => Promise<void>
  register: (email: string, password: string, displayName: string) => Promise<void>
  logout: () => void
  fetchMe: () => Promise<void>
  init: () => void
}

export const useAuthStore = create<AuthState>((set, get) => ({
  token: null,
  user: null,
  loading: false,
  error: null,

  init: () => {
    const token = getToken()
    if (token) {
      set({ token })
      get().fetchMe()
    }
  },

  login: async (email: string, password: string) => {
    set({ loading: true, error: null })
    try {
      const res = await apiFetch<{ token: string }>('/api/v1/auth/login', {
        method: 'POST',
        body: JSON.stringify({ email, password }),
      })
      setToken(res.token)
      set({ token: res.token, loading: false })
      await get().fetchMe()
    } catch (err: any) {
      set({ error: err.message, loading: false })
      throw err
    }
  },

  register: async (email: string, password: string, displayName: string) => {
    set({ loading: true, error: null })
    try {
      const res = await apiFetch<{ token: string }>('/api/v1/auth/register', {
        method: 'POST',
        body: JSON.stringify({ email, password, display_name: displayName }),
      })
      setToken(res.token)
      set({ token: res.token, loading: false })
      await get().fetchMe()
    } catch (err: any) {
      set({ error: err.message, loading: false })
      throw err
    }
  },

  logout: () => {
    clearToken()
    set({ token: null, user: null })
  },

  fetchMe: async () => {
    try {
      const user = await apiFetch<User>('/api/v1/auth/me')
      set({ user })
    } catch {
      set({ token: null, user: null })
      clearToken()
    }
  },
}))
