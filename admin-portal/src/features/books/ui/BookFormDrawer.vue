<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'

import AppIcon from '@/shared/ui/AppIcon.vue'
import type { Book, BookInput } from '../model/types'

const props = defineProps<{ open: boolean; book?: Book; saving: boolean }>()
const emit = defineEmits<{
  close: []
  save: [payload: BookInput]
}>()

const form = reactive({ title: '', author: '', isbn: '', price: '', stock: '' })
const validationError = ref('')
const heading = computed(() => (props.book ? 'Chỉnh sửa sách' : 'Thêm sách mới'))

watch(
  () => [props.open, props.book] as const,
  ([open, book]) => {
    if (!open) return
    form.title = book?.title ?? ''
    form.author = book?.author ?? ''
    form.isbn = book?.isbn ?? ''
    form.price = book ? (book.price_cents / 100).toFixed(2) : ''
    form.stock = book ? String(book.stock) : '0'
    validationError.value = ''
  },
  { immediate: true },
)

function close(): void {
  if (!props.saving) emit('close')
}

function submit(): void {
  const price = Number(form.price)
  const stock = Number(form.stock)
  if (!form.title.trim() || !form.author.trim() || !form.isbn.trim()) {
    validationError.value = 'Vui lòng nhập đầy đủ tên sách, tác giả và ISBN.'
    return
  }
  if (!Number.isFinite(price) || price < 0) {
    validationError.value = 'Giá sách phải là số không âm.'
    return
  }
  if (!Number.isInteger(stock) || stock < 0 || stock > 2_147_483_647) {
    validationError.value = 'Tồn kho phải là số nguyên không âm.'
    return
  }
  validationError.value = ''
  emit('save', {
    title: form.title.trim(),
    author: form.author.trim(),
    isbn: form.isbn.trim(),
    price_cents: Math.round(price * 100),
    stock,
  })
}
</script>

<template>
  <Teleport to="body">
    <Transition name="drawer">
      <div v-if="open" class="drawer-layer" role="presentation" @mousedown.self="close">
        <section class="drawer" role="dialog" aria-modal="true" :aria-label="heading">
          <header class="drawer__header">
            <div>
              <p class="eyebrow">Danh mục</p>
              <h2>{{ heading }}</h2>
            </div>
            <button class="icon-button" type="button" aria-label="Đóng" @click="close">
              <AppIcon name="close" />
            </button>
          </header>

          <form class="drawer__form" @submit.prevent="submit">
            <label>
              <span>Tên sách</span>
              <input
                v-model="form.title"
                maxlength="240"
                autocomplete="off"
                placeholder="Clean Architecture"
              />
            </label>
            <label>
              <span>Tác giả</span>
              <input
                v-model="form.author"
                maxlength="160"
                autocomplete="off"
                placeholder="Robert C. Martin"
              />
            </label>
            <label>
              <span>ISBN</span>
              <input
                v-model="form.isbn"
                maxlength="32"
                inputmode="numeric"
                autocomplete="off"
                placeholder="9780134494166"
              />
            </label>
            <div class="form-grid">
              <label>
                <span>Giá bán (USD)</span>
                <input v-model="form.price" type="number" min="0" step="0.01" placeholder="39.99" />
              </label>
              <label>
                <span>Tồn kho</span>
                <input v-model="form.stock" type="number" min="0" step="1" placeholder="10" />
              </label>
            </div>

            <p v-if="validationError" class="form-error">{{ validationError }}</p>
            <footer class="drawer__footer">
              <button
                class="button button--secondary"
                type="button"
                :disabled="saving"
                @click="close"
              >
                Hủy
              </button>
              <button class="button button--primary" type="submit" :disabled="saving">
                <span v-if="saving" class="spinner" aria-hidden="true" />
                <AppIcon v-else name="check" :size="17" />
                {{ saving ? 'Đang lưu…' : 'Lưu sách' }}
              </button>
            </footer>
          </form>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.drawer-layer {
  position: fixed;
  z-index: 80;
  inset: 0;
  display: flex;
  justify-content: flex-end;
  background: rgb(8 23 19 / 52%);
  backdrop-filter: blur(3px);
}
.drawer {
  width: min(520px, 100%);
  height: 100%;
  overflow-y: auto;
  padding: 28px;
  background: var(--color-panel);
  box-shadow: -24px 0 70px rgb(8 23 19 / 22%);
}
.drawer__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  padding-bottom: 24px;
  border-bottom: 1px solid var(--color-line);
}
.drawer__header h2 {
  margin: 8px 0 0;
  font-family: var(--font-display);
  font-size: 1.75rem;
}
.drawer__form {
  display: grid;
  gap: 20px;
  padding-top: 28px;
}
.drawer__form label {
  display: grid;
  gap: 8px;
  color: var(--color-ink);
  font-size: 0.78rem;
  font-weight: 750;
}
.drawer__form input {
  width: 100%;
  min-height: 48px;
  padding: 0 14px;
  border: 1px solid var(--color-line);
  border-radius: 10px;
  outline: none;
  color: var(--color-ink);
  background: white;
  transition: 150ms ease;
}
.drawer__form input:focus {
  border-color: var(--color-brand);
  box-shadow: 0 0 0 3px rgb(33 93 76 / 10%);
}
.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
}
.drawer__footer {
  display: flex;
  gap: 10px;
  justify-content: flex-end;
  padding-top: 10px;
}
.drawer-enter-active,
.drawer-leave-active {
  transition: opacity 180ms ease;
}
.drawer-enter-active .drawer,
.drawer-leave-active .drawer {
  transition: transform 220ms ease;
}
.drawer-enter-from,
.drawer-leave-to {
  opacity: 0;
}
.drawer-enter-from .drawer,
.drawer-leave-to .drawer {
  transform: translateX(100%);
}
@media (max-width: 520px) {
  .drawer {
    padding: 22px 18px;
  }
  .form-grid {
    grid-template-columns: 1fr;
  }
  .drawer__footer {
    display: grid;
    grid-template-columns: 1fr 1fr;
  }
}
</style>
