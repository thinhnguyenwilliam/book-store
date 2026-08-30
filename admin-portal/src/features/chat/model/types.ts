export interface Conversation {
  id: string
  customer_id: string
  type: 'support'
  status: 'open' | 'closed'
  last_message_sequence: number
  last_message_preview: string
  last_message_at?: string
  unread_count: number
  created_at: string
  updated_at: string
}

export interface ChatMessage {
  id: string
  conversation_id: string
  sender_id: string
  sender_name: string
  client_message_id: string
  sequence_number: number
  content: string
  message_type: 'text'
  created_at: string
  edited_at?: string
  deleted_at?: string
}

export interface CursorPage<T> {
  data: T[]
  pagination: { next_cursor?: string; has_more: boolean }
}

export interface ChatEvent<T = unknown> {
  type: string
  data: T
}
