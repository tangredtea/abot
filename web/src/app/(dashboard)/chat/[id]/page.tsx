'use client'

import { useEffect, useRef, useCallback } from 'react'
import { useParams } from 'next/navigation'
import { useChatStore } from '@/stores/chat'
import { ChatWebSocket, WSOutgoingMessage } from '@/lib/ws'
import MessageList from '@/components/chat/MessageList'
import MessageInput from '@/components/chat/MessageInput'
import StreamingText from '@/components/chat/StreamingText'

export default function ChatSessionPage() {
  const params = useParams()
  const sessionId = params.id as string
  const wsRef = useRef<ChatWebSocket | null>(null)

  const {
    loadSession,
    messages,
    streamingContent,
    isStreaming,
    addUserMessage,
    appendStreamDelta,
    finalizeStream,
    setStreaming,
    clearStreamingContent,
    currentSession,
  } = useChatStore()

  const handleWSMessage = useCallback((msg: WSOutgoingMessage) => {
    switch (msg.type) {
      case 'text_delta':
        appendStreamDelta(msg.content || '')
        break
      case 'tool_call':
        appendStreamDelta(`\n[Tool: ${msg.content}]\n`)
        break
      case 'tool_result':
        appendStreamDelta(`\n[Result: ${(msg.content || '').slice(0, 200)}]\n`)
        break
      case 'done':
        finalizeStream()
        break
      case 'error':
        appendStreamDelta(`\n[Error: ${msg.error}]\n`)
        finalizeStream()
        break
    }
  }, [appendStreamDelta, finalizeStream])

  useEffect(() => {
    loadSession(sessionId)
  }, [sessionId, loadSession])

  useEffect(() => {
    const ws = new ChatWebSocket(handleWSMessage, () => {
      setStreaming(false)
    })
    ws.connect()
    wsRef.current = ws

    return () => {
      ws.close()
    }
  }, [handleWSMessage, setStreaming])

  const handleSend = (content: string) => {
    if (!wsRef.current?.connected || !sessionId) return
    addUserMessage(content)
    clearStreamingContent()
    setStreaming(true)
    wsRef.current.send(sessionId, content)
  }

  const handleStop = () => {
    wsRef.current?.stop()
    finalizeStream()
  }

  return (
    <div className="flex-1 flex flex-col bg-white">
      {/* Header */}
      <div className="border-b bg-gradient-to-r from-blue-50 to-indigo-50 px-6 py-4">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold text-gray-800 truncate flex items-center gap-2">
            <span className="text-2xl">💬</span>
            {currentSession?.title || 'Chat'}
          </h2>
          <div className="flex items-center gap-2">
            <span className="text-xs text-gray-500 bg-white px-2 py-1 rounded-full">
              {messages.length} 条消息
            </span>
          </div>
        </div>
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-y-auto px-6 py-6 bg-gray-50">
        {messages.length === 0 && !streamingContent && (
          <div className="flex flex-col items-center justify-center h-full text-center">
            <div className="text-6xl mb-4">🤖</div>
            <h3 className="text-xl font-semibold text-gray-700 mb-2">
              开始对话
            </h3>
            <p className="text-gray-500 max-w-md">
              我是您的 AI 助手，可以帮您解答问题、编写代码、分析数据等。
            </p>
          </div>
        )}
        {messages.length > 0 && <MessageList messages={messages} />}
        {streamingContent && <StreamingText content={streamingContent} />}
      </div>

      {/* Input */}
      <div className="border-t bg-white px-6 py-4">
        <MessageInput
          onSend={handleSend}
          onStop={handleStop}
          isStreaming={isStreaming}
        />
      </div>
    </div>
  )
}
