import { getToken } from './auth'

export type WSMessageType = 'auth_ok' | 'text_delta' | 'tool_call' | 'tool_result' | 'done' | 'error'

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
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private shouldReconnect = true
  private reconnectAttempts = 0
  private authenticated = false
  private static MAX_RECONNECT_ATTEMPTS = 5

  constructor(onMessage: WSMessageHandler, onClose?: () => void) {
    this.onMessage = onMessage
    this.onClose = onClose
  }

  connect(): void {
    const token = getToken()
    if (!token) return

    this.shouldReconnect = true
    this.authenticated = false
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    const url = `${protocol}//${host}/api/v1/chat/ws`

    this.ws = new WebSocket(url)

    this.ws.onopen = () => {
      this.reconnectAttempts = 0
      // Authenticate via first message instead of URL query parameter
      this.ws?.send(JSON.stringify({ type: 'auth', token }))
    }

    this.ws.onmessage = (event) => {
      try {
        const msg: WSOutgoingMessage = JSON.parse(event.data)
        if (msg.type === 'auth_ok') {
          this.authenticated = true
          return
        }
        this.onMessage(msg)
      } catch {
        // ignore parse errors
      }
    }

    this.ws.onclose = () => {
      this.authenticated = false
      this.onClose?.()
      this.tryReconnect()
    }

    this.ws.onerror = () => {
      // Will trigger onclose
    }
  }

  private tryReconnect(): void {
    if (!this.shouldReconnect || this.reconnectAttempts >= ChatWebSocket.MAX_RECONNECT_ATTEMPTS) return
    this.reconnectAttempts++
    const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts - 1), 30000)
    this.reconnectTimer = setTimeout(() => this.connect(), delay)
  }

  send(sessionId: string, content: string): void {
    if (this.ws?.readyState === WebSocket.OPEN && this.authenticated) {
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
    this.shouldReconnect = false
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    this.ws?.close()
    this.ws = null
  }

  get connected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN && this.authenticated
  }
}
