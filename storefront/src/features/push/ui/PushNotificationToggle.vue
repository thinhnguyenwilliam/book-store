<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

import { useInboxStore } from '@/features/notifications/model/inbox.store'
import { useNotificationStore } from '@/features/notifications/model/notification.store'
import {
  onForegroundPush,
  requestAndEnablePush,
  supportsPush,
  synchronizePush,
  unregisterCurrentPushDevice,
} from '../lib/push'

const inbox = useInboxStore()
const notifications = useNotificationStore()
const supported = ref(false)
const enabled = ref(false)
const busy = ref(true)
const blocked = computed(() => supported.value && Notification.permission === 'denied')
let unsubscribe: () => void = () => undefined

onMounted(async () => {
  supported.value = await supportsPush()
  if (supported.value && Notification.permission === 'granted') {
    enabled.value = await synchronizePush().catch(() => false)
    unsubscribe = await onForegroundPush((payload) => {
      notifications.show(payload.notification?.title || 'Bạn có thông báo mới.', 'info')
      void inbox.refresh().catch(() => undefined)
    })
  }
  busy.value = false
})

onBeforeUnmount(() => unsubscribe())

async function toggle(): Promise<void> {
  busy.value = true
  try {
    if (enabled.value) {
      await unregisterCurrentPushDevice()
      enabled.value = false
      notifications.show('Đã tắt thông báo đẩy.', 'info')
      return
    }
    enabled.value = await requestAndEnablePush()
    if (enabled.value) {
      unsubscribe()
      unsubscribe = await onForegroundPush((payload) => {
        notifications.show(payload.notification?.title || 'Bạn có thông báo mới.', 'info')
        void inbox.refresh().catch(() => undefined)
      })
      notifications.show('Đã bật thông báo đẩy.', 'success')
    }
  } catch {
    notifications.show('Không thể cập nhật thông báo đẩy.', 'error')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div v-if="supported" class="push-toggle">
    <span>{{
      blocked ? 'Push đã bị chặn trong trình duyệt' : 'Thông báo khi không mở trang'
    }}</span>
    <button type="button" :disabled="busy || blocked" @click="toggle">
      {{ busy ? 'Đang xử lý…' : enabled ? 'Tắt push' : 'Bật push' }}
    </button>
  </div>
</template>

<style scoped>
.push-toggle {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  padding: 11px 18px;
  border-bottom: 1px solid var(--color-line);
  color: var(--color-muted);
  font-size: 0.72rem;
}
.push-toggle button {
  flex: 0 0 auto;
  padding: 5px 9px;
  border: 1px solid var(--color-line);
  border-radius: 999px;
  color: var(--color-brand);
  background: transparent;
  font-size: 0.68rem;
  font-weight: 750;
  cursor: pointer;
}
.push-toggle button:disabled {
  opacity: 0.5;
  cursor: default;
}
</style>
