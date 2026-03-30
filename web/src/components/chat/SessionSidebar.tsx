'use client'

import { useEffect } from 'react'
import { useRouter, useParams } from 'next/navigation'
import { useChatStore, ChatSession } from '@/stores/chat'

export default function SessionSidebar() {
  const router = useRouter()
  const params = useParams()
  const currentId = params?.id as string | undefined

  const { sessions, fetchSessions, deleteSession } = useChatStore()

  useEffect(() => {
    fetchSessions()
  }, [fetchSessions])

  const handleNewChat = () => {
    router.push('/chat')
  }

  // Group sessions by agent_id
  const sessionsByAgent = sessions.reduce((acc, session) => {
    const agentId = session.agent_id || 'unknown'
    if (!acc[agentId]) {
      acc[agentId] = []
    }
    acc[agentId].push(session)
    return acc
  }, {} as Record<string, ChatSession[]>)

  return (
    <div className="py-2">
      <div className="px-3 mb-3">
        <button
          onClick={handleNewChat}
          className="w-full py-2.5 px-3 bg-primary text-white text-sm rounded-lg hover:bg-primary-hover transition-colors duration-150 font-medium shadow-sm flex items-center justify-center gap-2"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
          </svg>
          <span>新建对话</span>
        </button>
      </div>

      {sessions.length === 0 ? (
        <div className="px-3 py-8 text-sm text-gray-500 text-center">
          <div className="text-3xl mb-2">💬</div>
          <p>还没有对话</p>
          <p className="text-xs mt-1">点击上方按钮开始</p>
        </div>
      ) : (
        <div className="space-y-4">
          {Object.entries(sessionsByAgent).map(([agentId, agentSessions]) => (
            <div key={agentId}>
              <div className="px-3 py-1.5 text-xs text-gray-400 uppercase font-semibold flex items-center gap-2">
                <span>🤖</span>
                <span>Agent: {agentId.slice(0, 8)}</span>
              </div>
              <div className="space-y-0.5">
                {agentSessions.map((s) => (
                  <div
                    key={s.id}
                    onClick={() => router.push(`/chat/${s.id}`)}
                    role="button"
                    tabIndex={0}
                    onKeyDown={(e) => { if (e.key === 'Enter') router.push(`/chat/${s.id}`) }}
                    className={`w-full text-left px-3 py-2 text-sm rounded-lg hover:bg-gray-800 group flex items-center gap-2 transition-colors cursor-pointer ${
                      currentId === s.id ? 'bg-gray-800 text-white' : 'text-gray-300'
                    }`}
                  >
                    <span className="flex-1 truncate">{s.title}</span>
                    {s.pinned && (
                      <span className="text-yellow-400 text-xs">📌</span>
                    )}
                    <button
                      onClick={(e) => {
                        e.stopPropagation()
                        if (confirm('确定要删除这个对话吗？')) {
                          deleteSession(s.id)
                        }
                      }}
                      className="opacity-0 group-hover:opacity-100 text-gray-500 hover:text-red-400 transition-opacity"
                      title="删除对话"
                    >
                      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                      </svg>
                    </button>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
