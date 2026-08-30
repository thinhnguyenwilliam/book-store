import { apiRequest } from '@/shared/api/http-client'
import type { ChatMessage, Conversation, CursorPage } from '../model/types'

export function openSupportConversation(): Promise<Conversation> {
  return apiRequest('/api/v1/chat/conversations/support', { method: 'POST' })
}

export function listConversations(cursor = ''): Promise<CursorPage<Conversation>> {
  return apiRequest('/api/v1/chat/conversations', {
    params: { limit: 20, cursor: cursor || undefined },
  })
}

export function listMessages(
  conversationId: string,
  cursor = '',
): Promise<CursorPage<ChatMessage>> {
  return apiRequest(`/api/v1/chat/conversations/${conversationId}/messages`, {
    params: { limit: 30, cursor: cursor || undefined },
  })
}

export function sendMessage(
  conversationId: string,
  content: string,
  clientMessageId: string,
): Promise<ChatMessage> {
  return apiRequest(`/api/v1/chat/conversations/${conversationId}/messages`, {
    method: 'POST',
    data: { content, client_message_id: clientMessageId },
  })
}

export function markRead(
  conversationId: string,
  sequenceNumber: number,
): Promise<{ last_read_sequence: number }> {
  return apiRequest(`/api/v1/chat/conversations/${conversationId}/read`, {
    method: 'PUT',
    data: { sequence_number: sequenceNumber },
  })
}

export function unreadCount(): Promise<{ count: number }> {
  return apiRequest('/api/v1/chat/unread-count')
}

export function issueWebSocketTicket(): Promise<{ ticket: string; expires_in: number }> {
  return apiRequest('/api/v1/chat/ws-ticket', { method: 'POST' })
}
