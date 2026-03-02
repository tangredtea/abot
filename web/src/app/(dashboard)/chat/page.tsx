'use client'

import { useState, useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { getToken } from '@/lib/auth'
import { useChatStore } from '@/stores/chat'

interface Agent {
  id: string
  name: string
  description: string
  avatar: string
  model: string
  status: string
}

export default function ChatPage() {
  const router = useRouter()
  const [agents, setAgents] = useState<Agent[]>([])
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState<string | null>(null)
  const createSession = useChatStore((s) => s.createSession)

  useEffect(() => {
    fetchAgents()
  }, [])

  const fetchAgents = async () => {
    try {
      const token = getToken()
      const res = await fetch('http://localhost:3001/api/v1/agents', {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      })
      if (res.ok) {
        const data = await res.json()
        setAgents(data || [])
      }
    } catch (error) {
      console.error('Failed to fetch agents:', error)
    } finally {
      setLoading(false)
    }
  }

  const handleStartChat = async (agentId: string) => {
    if (creating) return
    setCreating(agentId)
    try {
      const session = await createSession(agentId)
      router.push(`/chat/${session.id}`)
    } catch (error) {
      console.error('Failed to create session:', error)
      alert('创建会话失败')
    } finally {
      setCreating(null)
    }
  }

  if (loading) {
    return (
      <div className="flex-1 flex items-center justify-center bg-gray-50">
        <div className="text-gray-500">加载中...</div>
      </div>
    )
  }

  if (agents.length === 0) {
    return (
      <div className="flex-1 flex items-center justify-center bg-gray-50">
        <div className="text-center max-w-md px-6">
          <div className="text-6xl mb-6">🤖</div>
          <h1 className="text-2xl font-bold text-gray-900 mb-3">还没有 Agent</h1>
          <p className="text-gray-600 mb-8">
            请先创建一个 Agent，然后就可以开始对话了
          </p>
          <button
            onClick={() => router.push('/agents')}
            className="inline-flex items-center gap-2 px-6 py-3 bg-primary text-white rounded-lg hover:bg-primary-hover transition-colors duration-150 font-medium shadow-sm"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
            </svg>
            <span>创建 Agent</span>
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="flex-1 flex flex-col bg-gray-50 overflow-hidden">
      {/* Header */}
      <div className="bg-white border-b shadow-sm">
        <div className="px-4 sm:px-6 lg:px-8 py-4 sm:py-5">
          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-xl lg:text-2xl font-bold text-gray-900">选择 Agent 开始对话</h1>
              <p className="text-sm text-gray-500 mt-1">选择一个 Agent 创建新的对话会话</p>
            </div>
            <button
              onClick={() => router.push('/agents')}
              className="inline-flex items-center gap-2 px-4 py-2 border border-gray-300 text-gray-700 rounded-lg hover:bg-gray-50 transition-colors duration-150 font-medium"
            >
              <span>管理 Agents</span>
            </button>
          </div>
        </div>
      </div>

      {/* Agent Grid */}
      <div className="flex-1 overflow-y-auto px-4 sm:px-6 lg:px-8 py-6">
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {agents.map((agent) => (
            <button
              key={agent.id}
              onClick={() => handleStartChat(agent.id)}
              disabled={creating === agent.id}
              className="bg-white rounded-xl border-2 border-gray-200 p-6 hover:border-primary hover:shadow-lg transition-all duration-200 text-left disabled:opacity-50 disabled:cursor-not-allowed group"
            >
              <div className="flex items-start gap-4">
                <div className="text-5xl flex-shrink-0 group-hover:scale-110 transition-transform duration-200">
                  {agent.avatar || '🤖'}
                </div>
                <div className="flex-1 min-w-0">
                  <h3 className="font-semibold text-gray-900 text-lg mb-1 truncate">
                    {agent.name}
                  </h3>
                  <p className="text-sm text-gray-500 line-clamp-2 mb-3">
                    {agent.description || '暂无描述'}
                  </p>
                  <div className="flex items-center gap-2 text-xs">
                    <span className="px-2 py-1 bg-blue-50 text-blue-700 rounded-full">
                      {agent.model}
                    </span>
                    <span className={`px-2 py-1 rounded-full ${
                      agent.status === 'active' 
                        ? 'bg-green-50 text-green-700' 
                        : 'bg-gray-50 text-gray-700'
                    }`}>
                      {agent.status === 'active' ? '运行中' : '已停用'}
                    </span>
                  </div>
                </div>
              </div>
              
              {creating === agent.id && (
                <div className="mt-4 flex items-center justify-center gap-2 text-primary">
                  <div className="w-4 h-4 border-2 border-primary border-t-transparent rounded-full animate-spin"></div>
                  <span className="text-sm">创建会话中...</span>
                </div>
              )}
              
              {creating !== agent.id && (
                <div className="mt-4 flex items-center justify-center gap-2 text-primary opacity-0 group-hover:opacity-100 transition-opacity">
                  <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
                  </svg>
                  <span className="text-sm font-medium">开始对话</span>
                </div>
              )}
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}
