import { env } from '@/shared/config/env'
import { issueWebSocketTicket } from '../api/chat.api'
import type { ChatEvent } from '../model/types'

export class ChatSocket {
  private socket: WebSocket | undefined
  private intentionallyClosed = false

  async connect(onEvent: (event: ChatEvent) => void, onDisconnect: () => void): Promise<void> {
    this.intentionallyClosed = false
    const { ticket } = await issueWebSocketTicket()
    const url = new URL(env.apiBaseUrl)
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
    url.pathname = `${url.pathname.replace(/\/$/, '')}/api/v1/chat/ws`
    url.search = new URLSearchParams({ ticket }).toString()
    await new Promise<void>((resolve, reject) => {
      const socket = new WebSocket(url)
      this.socket = socket
      socket.onopen = () => resolve()
      socket.onerror = () => reject(new Error('WebSocket connection failed'))
      socket.onmessage = (message) => {
        try {
          onEvent(JSON.parse(String(message.data)) as ChatEvent)
        } catch {
          // REST resync remains authoritative after reconnect.
        }
      }
      socket.onclose = () => {
        this.socket = undefined
        if (!this.intentionallyClosed) onDisconnect()
      }
    })
  }

  send(type: string, data: unknown): boolean {
    if (this.socket?.readyState !== WebSocket.OPEN) return false
    this.socket.send(JSON.stringify({ type, request_id: crypto.randomUUID(), data }))
    return true
  }

  close(): void {
    this.intentionallyClosed = true
    this.socket?.close(1000, 'client closed')
    this.socket = undefined
  }
}
