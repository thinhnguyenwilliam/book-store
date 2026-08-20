import { ref } from 'vue'
import { defineStore } from 'pinia'

export interface Notification {
  id: number
  message: string
  tone: 'success' | 'error' | 'info'
}

export const useNotificationStore = defineStore('notifications', () => {
  const current = ref<Notification | null>(null)
  let nextID = 1
  let timeoutID: number | undefined

  function show(message: string, tone: Notification['tone'] = 'info'): void {
    window.clearTimeout(timeoutID)
    current.value = { id: nextID++, message, tone }
    timeoutID = window.setTimeout(() => {
      current.value = null
    }, 3200)
  }

  function dismiss(): void {
    window.clearTimeout(timeoutID)
    current.value = null
  }

  return { current, show, dismiss }
})
