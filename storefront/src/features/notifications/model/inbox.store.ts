import { ref } from 'vue'
import { defineStore } from 'pinia'

import * as api from '../api/notifications.api'

export interface Notification {
  id: string
  type: string
  title: string
  body: string
  data: Record<string, unknown>
  read_at?: string
  created_at: string
}

export interface NotificationListResponse {
  data: Notification[]
  pagination: { next_cursor?: string; has_more: boolean }
}

export const useInboxStore = defineStore('notification-inbox', () => {
  const items = ref<Notification[]>([])
  const unreadCount = ref(0)
  const loading = ref(false)

  async function refresh(): Promise<void> {
    loading.value = true
    try {
      const [page, unread] = await Promise.all([api.listNotifications(), api.getUnreadCount()])
      items.value = page.data
      unreadCount.value = unread.count
    } finally {
      loading.value = false
    }
  }

  async function read(item: Notification): Promise<void> {
    if (item.read_at) return
    const updated = await api.markRead(item.id)
    const index = items.value.findIndex((current) => current.id === item.id)
    if (index >= 0) items.value[index] = updated
    unreadCount.value = Math.max(0, unreadCount.value - 1)
  }

  async function readAll(): Promise<void> {
    if (!unreadCount.value) return
    await api.markAllRead()
    const now = new Date().toISOString()
    items.value = items.value.map((item) => ({ ...item, read_at: item.read_at || now }))
    unreadCount.value = 0
  }

  function clear(): void {
    items.value = []
    unreadCount.value = 0
  }

  return { items, unreadCount, loading, refresh, read, readAll, clear }
})
