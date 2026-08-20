<script setup lang="ts">
import AppIcon from './AppIcon.vue'

defineProps<{ open: boolean; title: string; message: string; busy?: boolean }>()
const emit = defineEmits<{ cancel: []; confirm: [] }>()
</script>

<template>
  <Teleport to="body">
    <Transition name="dialog">
      <div v-if="open" class="dialog-layer" @mousedown.self="emit('cancel')">
        <section class="dialog" role="alertdialog" aria-modal="true" :aria-label="title">
          <span class="dialog__icon"><AppIcon name="alert" :size="22" /></span>
          <h2>{{ title }}</h2>
          <p>{{ message }}</p>
          <div class="dialog__actions">
            <button
              class="button button--secondary"
              type="button"
              :disabled="busy"
              @click="emit('cancel')"
            >
              Hủy
            </button>
            <button
              class="button button--danger"
              type="button"
              :disabled="busy"
              @click="emit('confirm')"
            >
              <span v-if="busy" class="spinner" />
              <AppIcon v-else name="trash" :size="17" />
              {{ busy ? 'Đang xóa…' : 'Xóa sách' }}
            </button>
          </div>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.dialog-layer {
  position: fixed;
  z-index: 90;
  inset: 0;
  display: grid;
  place-items: center;
  padding: 20px;
  background: rgb(8 23 19 / 55%);
  backdrop-filter: blur(4px);
}
.dialog {
  width: min(430px, 100%);
  padding: 28px;
  border-radius: 18px;
  background: white;
  box-shadow: 0 25px 80px rgb(8 23 19 / 30%);
  text-align: center;
}
.dialog__icon {
  display: grid;
  width: 48px;
  height: 48px;
  margin: 0 auto 16px;
  place-items: center;
  border-radius: 14px;
  color: var(--color-danger);
  background: #fff0ed;
}
.dialog h2 {
  margin: 0;
  font-family: var(--font-display);
  font-size: 1.45rem;
}
.dialog p {
  margin: 10px 0 24px;
  color: var(--color-muted);
  font-size: 0.9rem;
  line-height: 1.6;
}
.dialog__actions {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}
.dialog-enter-active,
.dialog-leave-active {
  transition: 160ms ease;
}
.dialog-enter-from,
.dialog-leave-to {
  opacity: 0;
  transform: scale(0.98);
}
</style>
