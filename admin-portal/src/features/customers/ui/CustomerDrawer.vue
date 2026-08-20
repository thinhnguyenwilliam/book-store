<script setup lang="ts">
import { reactive, ref, watch } from 'vue'

import { formatDateTime } from '@/shared/lib/format'
import AppIcon from '@/shared/ui/AppIcon.vue'
import type { Customer, CustomerInput } from '../model/types'

const props = defineProps<{ open: boolean; customer?: Customer; saving: boolean }>()
const emit = defineEmits<{ close: []; save: [payload: CustomerInput] }>()
const form = reactive({ displayName: '' })
const validationError = ref('')

watch(
  () => [props.open, props.customer] as const,
  ([open, customer]) => {
    if (!open) return
    form.displayName = customer?.display_name ?? ''
    validationError.value = ''
  },
  { immediate: true },
)

function close(): void {
  if (!props.saving) emit('close')
}

function submit(): void {
  const displayName = form.displayName.trim()
  if (!displayName) {
    validationError.value = 'Tên hiển thị không được để trống.'
    return
  }
  validationError.value = ''
  emit('save', { display_name: displayName })
}
</script>

<template>
  <Teleport to="body">
    <Transition name="customer-drawer">
      <div v-if="open" class="customer-drawer-layer" role="presentation" @mousedown.self="close">
        <section class="customer-drawer" role="dialog" aria-modal="true" aria-label="Khách hàng">
          <header>
            <div>
              <p class="eyebrow">Customer profile</p>
              <h2>Chi tiết khách hàng</h2>
            </div>
            <button class="icon-button" type="button" aria-label="Đóng" @click="close">
              <AppIcon name="close" />
            </button>
          </header>

          <dl v-if="customer" class="customer-facts">
            <div>
              <dt>Email</dt>
              <dd>{{ customer.email }}</dd>
            </div>
            <div>
              <dt>Customer ID</dt>
              <dd>
                <code>{{ customer.id }}</code>
              </dd>
            </div>
            <div>
              <dt>Ngày tham gia</dt>
              <dd>{{ formatDateTime(customer.created_at) }}</dd>
            </div>
            <div>
              <dt>Cập nhật gần nhất</dt>
              <dd>{{ formatDateTime(customer.updated_at) }}</dd>
            </div>
          </dl>

          <form @submit.prevent="submit">
            <label>
              <span>Tên hiển thị</span>
              <input v-model="form.displayName" maxlength="120" autocomplete="off" />
            </label>
            <p v-if="validationError" class="form-error">{{ validationError }}</p>
            <footer>
              <button
                class="button button--secondary"
                type="button"
                :disabled="saving"
                @click="close"
              >
                Hủy
              </button>
              <button class="button button--primary" type="submit" :disabled="saving">
                <span v-if="saving" class="spinner" />
                <AppIcon v-else name="check" :size="17" />
                {{ saving ? 'Đang lưu…' : 'Lưu thay đổi' }}
              </button>
            </footer>
          </form>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.customer-drawer-layer {
  position: fixed;
  z-index: 80;
  inset: 0;
  display: flex;
  justify-content: flex-end;
  background: rgb(8 23 19 / 52%);
  backdrop-filter: blur(3px);
}
.customer-drawer {
  width: min(520px, 100%);
  height: 100%;
  overflow-y: auto;
  padding: 28px;
  background: var(--color-panel);
  box-shadow: -24px 0 70px rgb(8 23 19 / 22%);
}
.customer-drawer > header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  padding-bottom: 24px;
  border-bottom: 1px solid var(--color-line);
}
.customer-drawer h2 {
  margin: 8px 0 0;
  font-family: var(--font-display);
  font-size: 1.75rem;
}
.customer-facts {
  display: grid;
  gap: 0;
  margin: 24px 0;
  border: 1px solid var(--color-line);
  border-radius: 12px;
}
.customer-facts div {
  display: grid;
  gap: 6px;
  padding: 14px;
  border-bottom: 1px solid var(--color-line);
}
.customer-facts div:last-child {
  border-bottom: 0;
}
.customer-facts dt {
  color: var(--color-muted);
  font-size: 0.68rem;
  font-weight: 750;
}
.customer-facts dd {
  overflow-wrap: anywhere;
  margin: 0;
  font-size: 0.8rem;
}
.customer-facts code {
  font-size: 0.7rem;
}
form {
  display: grid;
  gap: 18px;
}
form label {
  display: grid;
  gap: 8px;
  font-size: 0.77rem;
  font-weight: 750;
}
form input {
  width: 100%;
  min-height: 48px;
  padding: 0 14px;
  border: 1px solid var(--color-line);
  border-radius: 10px;
  outline: none;
}
form input:focus {
  border-color: var(--color-brand);
  box-shadow: 0 0 0 3px rgb(33 93 76 / 10%);
}
form footer {
  display: flex;
  gap: 10px;
  justify-content: flex-end;
}
.customer-drawer-enter-active,
.customer-drawer-leave-active {
  transition: opacity 180ms ease;
}
.customer-drawer-enter-active .customer-drawer,
.customer-drawer-leave-active .customer-drawer {
  transition: transform 220ms ease;
}
.customer-drawer-enter-from,
.customer-drawer-leave-to {
  opacity: 0;
}
.customer-drawer-enter-from .customer-drawer,
.customer-drawer-leave-to .customer-drawer {
  transform: translateX(100%);
}
@media (max-width: 520px) {
  .customer-drawer {
    padding: 22px 18px;
  }
  form footer {
    display: grid;
    grid-template-columns: 1fr 1fr;
  }
}
</style>
