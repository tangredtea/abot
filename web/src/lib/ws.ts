import { getToken } from './auth'

export type WSMessageType = 'text_delta' | 'tool_call' | 'tool_result' | 'done' | 'error'

export interface WSOutgoingMessage {
  type: WSMessageType
  content?: string
  args?: Record<string, any>
  error?: string
}

export type WSMessageHandler = (msg: WSOutgoingMessage) => void

export class ChatWebSocket {
  private ws: WebSocket | null = null
  private onMessage: WSMessageHandler
  private onClose?: () => void

  constructor(onMessage: WSMessageHandler, onClose?: () => void) {
    this.onMessage = onMessage
    this.onClose = onClose
  }

  connect(): void {
    const token = getToken()
    if (!token) return

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    // Always connect to backend on port 3001
    const url = `${protocol}//localhost:3001/api/v1/chat/ws?token=${encodeURIComponent(token)}`

    this.ws = new WebSocket(url)

    this.ws.onmessage = (event) => {
      try {
        const msg: WSOutgoingMessage = JSON.parse(event.data)
        this.onMessage(msg)
      } catch {
        // ignore parse errors
      }
    }

    this.ws.onclose = () => {
      this.onClose?.()
    }

    this.ws.onerror = () => {
      // Will trigger onclose
    }
  }

  send(sessionId: string, content: string): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({
        type: 'message',
        session_id: sessionId,
        content,
      }))
    }
  }

  stop(): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ type: 'stop' }))
    }
  }

  close(): void {
    this.ws?.close()
    this.ws = null
  }

  get connected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN
  }
}
