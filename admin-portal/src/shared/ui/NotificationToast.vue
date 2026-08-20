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
    >
      <span class="toast__icon"><AppIcon name="check" :size="16" /></span>
      <span>{{ notifications.current.message }}</span>
      <button type="button" aria-label="Đóng thông báo" @click="notifications.dismiss">
        <AppIcon name="close" :size="16" />
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
  display: flex;
  max-width: min(430px, calc(100vw - 32px));
  min-height: 54px;
  gap: 12px;
  align-items: center;
  padding: 12px 14px;
  border: 1px solid #c8ded5;
  border-radius: 14px;
  color: #15362d;
  background: #f8fffb;
  box-shadow: 0 18px 55px rgb(15 36 29 / 18%);
  font-size: 0.88rem;
  font-weight: 650;
}
.toast--danger {
  border-color: #efc6bf;
  color: #873c32;
  background: #fff9f7;
}
.toast--info {
  border-color: #cbd8e8;
  color: #264e74;
  background: #f7fbff;
}
.toast__icon {
  display: grid;
  width: 28px;
  height: 28px;
  place-items: center;
  border-radius: 50%;
  color: white;
  background: #2c735f;
}
.toast button {
  display: grid;
  margin-left: auto;
  padding: 4px;
  border: 0;
  color: inherit;
  background: transparent;
  cursor: pointer;
}
.toast-enter-active,
.toast-leave-active {
  transition: 180ms ease;
}
.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateY(12px);
}
</style>
