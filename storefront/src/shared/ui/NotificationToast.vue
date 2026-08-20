<script setup lang="ts">
import { useNotificationStore } from '@/features/notifications/model/notification.store'
import AppIcon from './AppIcon.vue'

const notifications = useNotificationStore()
</script>

<template>
  <Transition name="toast">
    <div
      v-if="notifications.current"
      class="toast"
      :class="`toast--${notifications.current.tone}`"
      role="status"
      aria-live="polite"
    >
      <AppIcon :name="notifications.current.tone === 'success' ? 'check' : 'sparkles'" />
      <span>{{ notifications.current.message }}</span>
      <button type="button" aria-label="Đóng thông báo" @click="notifications.dismiss">
        <AppIcon name="close" :size="18" />
      </button>
    </div>
  </Transition>
</template>

<style scoped>
.toast {
  position: fixed;
  z-index: 100;
  right: 24px;
  bottom: 24px;
  display: grid;
  grid-template-columns: auto 1fr auto;
  gap: 12px;
  align-items: center;
  max-width: min(420px, calc(100vw - 32px));
  padding: 14px 16px;
  border: 1px solid rgb(255 255 255 / 16%);
  border-radius: 14px;
  color: white;
  background: var(--color-brand);
  box-shadow: var(--shadow-lg);
}
.toast--error {
  background: #8b352e;
}
.toast--info {
  background: #355b78;
}
.toast button {
  display: grid;
  padding: 0;
  border: 0;
  color: inherit;
  background: transparent;
  cursor: pointer;
  opacity: 0.72;
}
.toast-enter-active,
.toast-leave-active {
  transition: 220ms ease;
}
.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateY(12px);
}
@media (max-width: 600px) {
  .toast {
    right: 16px;
    bottom: 16px;
    left: 16px;
  }
}
</style>
