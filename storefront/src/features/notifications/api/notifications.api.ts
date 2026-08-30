import { apiRequest } from '@/shared/api/http-client'

import type { Notification, NotificationListResponse } from '../model/inbox.store'

export function listNotifications(limit = 8): Promise<NotificationListResponse> {
  return apiRequest('/api/v1/notifications', { params: { limit } })
}

export function getUnreadCount(): Promise<{ count: number }> {
  return apiRequest('/api/v1/notifications/unread-count')
}

export function markRead(id: string): Promise<Notification> {
  return apiRequest(`/api/v1/notifications/${encodeURIComponent(id)}/read`, { method: 'PUT' })
}

export function markAllRead(): Promise<{ updated: number }> {
  return apiRequest('/api/v1/notifications/read-all', { method: 'PUT' })
}
