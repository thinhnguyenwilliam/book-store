<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

import { useNotificationStore } from '@/features/notifications/model/notification.store'
import {
  onForegroundPush,
  requestAndEnablePush,
  supportsPush,
  synchronizePush,
  unregisterCurrentPushDevice,
} from '../lib/push'

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
      notifications.show(payload.notification?.title || 'Có thông báo quản trị mới.', 'info')
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
        notifications.show(payload.notification?.title || 'Có thông báo quản trị mới.', 'info')
      })
      notifications.show('Đã bật thông báo đẩy.')
    }
  } catch {
    notifications.show('Không thể cập nhật thông báo đẩy.', 'danger')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <button
    v-if="supported"
    class="push-toggle"
    type="button"
    :disabled="busy || blocked"
    :title="blocked ? 'Push đã bị chặn trong trình duyệt' : undefined"
    @click="toggle"
  >
    {{ busy ? 'Đang xử lý…' : blocked ? 'Push bị chặn' : enabled ? 'Tắt push' : 'Bật push' }}
  </button>
</template>

<style scoped>
.push-toggle {
  min-height: 38px;
  padding: 0 13px;
  border: 1px solid var(--color-line);
  border-radius: 9px;
  color: var(--color-brand);
  background: white;
  font-size: 0.76rem;
  font-weight: 750;
  cursor: pointer;
}
.push-toggle:disabled {
  opacity: 0.55;
  cursor: default;
}
@media (max-width: 620px) {
  .push-toggle {
    width: 40px;
    overflow: hidden;
    padding: 0;
    font-size: 0;
  }
  .push-toggle::after {
    content: '🔔';
    font-size: 1rem;
  }
}
</style>
