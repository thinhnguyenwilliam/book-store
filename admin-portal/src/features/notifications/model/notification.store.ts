import { ref } from 'vue'
import { defineStore } from 'pinia'

type NotificationTone = 'success' | 'danger' | 'info'

interface Notification {
  id: number
  message: string
  tone: NotificationTone
}

export const useNotificationStore = defineStore('admin-notifications', () => {
  const current = ref<Notification>()
  let sequence = 0
  let timer: number | undefined

  function show(message: string, tone: NotificationTone = 'success'): void {
    window.clearTimeout(timer)
    current.value = { id: ++sequence, message, tone }
    timer = window.setTimeout(dismiss, 3600)
  }

  function dismiss(): void {
    current.value = undefined
    window.clearTimeout(timer)
  }

  return { current, show, dismiss }
})
