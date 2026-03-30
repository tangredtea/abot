import { create } from 'zustand'
import { apiFetch } from '@/lib/api'

export interface ChatSession {
  id: string
  tenant_id: string
  agent_id: string
  title: string
  pinned: boolean
  archived: boolean
  created_at: string
  updated_at: string
}

export interface Message {
  role: string
  content: string
}

interface ChatState {
  sessions: ChatSession[]
  currentSession: ChatSession | null
  messages: Message[]
  streamingContent: string
  isStreaming: boolean
  loadingSessions: boolean

  fetchSessions: () => Promise<void>
  createSession: (agentId?: string) => Promise<ChatSession>
  loadSession: (id: string) => Promise<void>
  updateSession: (id: string, data: Partial<Pick<ChatSession, 'title' | 'pinned' | 'archived'>>) => Promise<void>
  deleteSession: (id: string) => Promise<void>

  addUserMessage: (content: string) => void
  appendStreamDelta: (delta: string) => void
  finalizeStream: () => void
  setStreaming: (v: boolean) => void
  clearStreamingContent: () => void
}

export const useChatStore = create<ChatState>((set, get) => ({
  sessions: [],
  currentSession: null,
  messages: [],
  streamingContent: '',
  isStreaming: false,
  loadingSessions: false,

  fetchSessions: async () => {
    set({ loadingSessions: true })
    try {
      const sessions = await apiFetch<ChatSession[]>('/api/v1/sessions')
      set({ sessions, loadingSessions: false })
    } catch {
      set({ loadingSessions: false })
    }
  },

  createSession: async (agentId?: string) => {
    const body: any = {}
    if (agentId) body.agent_id = agentId
    const session = await apiFetch<ChatSession>('/api/v1/sessions', {
      method: 'POST',
      body: JSON.stringify(body),
    })
    set((s) => ({ sessions: [session, ...s.sessions] }))
    return session
  },

  loadSession: async (id: string) => {
    const data = await apiFetch<ChatSession & { messages: Message[] }>(`/api/v1/sessions/${id}`)
    set({
      currentSession: data,
      messages: data.messages || [],
      streamingContent: '',
      isStreaming: false,
    })
  },

  updateSession: async (id: string, data) => {
    await apiFetch(`/api/v1/sessions/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    })
    await get().fetchSessions()
  },

  deleteSession: async (id: string) => {
    await apiFetch(`/api/v1/sessions/${id}`, { method: 'DELETE' })
    set((s) => ({
      sessions: s.sessions.filter((sess) => sess.id !== id),
      currentSession: s.currentSession?.id === id ? null : s.currentSession,
      messages: s.currentSession?.id === id ? [] : s.messages,
    }))
  },

  addUserMessage: (content: string) => {
    set((s) => ({
      messages: [...s.messages, { role: 'user', content }],
    }))
  },

  appendStreamDelta: (delta: string) => {
    set((s) => ({ streamingContent: s.streamingContent + delta }))
  },

  finalizeStream: () => {
    set((s) => ({
      messages: s.streamingContent
        ? [...s.messages, { role: 'model', content: s.streamingContent }]
        : s.messages,
      streamingContent: '',
      isStreaming: false,
    }))
  },

  setStreaming: (v: boolean) => set({ isStreaming: v }),
  clearStreamingContent: () => set({ streamingContent: '' }),
}))
