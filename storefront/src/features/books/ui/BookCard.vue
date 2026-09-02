<script setup lang="ts">
import { useCartStore } from '@/features/cart/model/cart.store'
import { useAuthStore } from '@/features/auth/model/auth.store'
import { trackBookAddedToCart } from '@/features/analytics/lib/customer-activity'
import { useNotificationStore } from '@/features/notifications/model/notification.store'
import { formatPrice } from '@/shared/lib/format'
import AppIcon from '@/shared/ui/AppIcon.vue'

import type { Book } from '../model/types'
import BookCover from './BookCover.vue'

const props = defineProps<{ book: Book }>()
const cart = useCartStore()
const auth = useAuthStore()
const notifications = useNotificationStore()

async function addToCart(): Promise<void> {
  try {
    await cart.add(props.book, auth.isAuthenticated)
    trackBookAddedToCart(props.book.id)
    notifications.show(`Đã thêm “${props.book.title}” vào giỏ.`, 'success')
  } catch {
    notifications.show('Không thể cập nhật giỏ hàng. Vui lòng thử lại.', 'error')
  }
}
</script>

<template>
  <article class="book-card">
    <RouterLink :to="`/sach/${book.id}`" class="book-card__cover" :aria-label="`Xem ${book.title}`">
      <BookCover :title="book.title" :author="book.author" :isbn="book.isbn" />
    </RouterLink>
    <div class="book-card__body">
      <p class="book-card__eyebrow">
        {{ book.stock > 0 ? `Còn ${book.stock} cuốn` : 'Tạm hết hàng' }}
      </p>
      <RouterLink :to="`/sach/${book.id}`" class="book-card__title">{{ book.title }}</RouterLink>
      <p class="book-card__author">{{ book.author }}</p>
      <div class="book-card__footer">
        <strong>{{ formatPrice(book.price_cents) }}</strong>
        <button
          type="button"
          :disabled="book.stock < 1"
          aria-label="Thêm vào giỏ"
          @click="addToCart"
        >
          <AppIcon name="plus" :size="17" />
        </button>
      </div>
    </div>
  </article>
</template>

<style scoped>
.book-card {
  display: flex;
  min-width: 0;
  flex-direction: column;
}
.book-card__cover {
  display: grid;
  min-height: 310px;
  place-items: center;
  padding: 28px 24px;
  border-radius: var(--radius-lg);
  background: #e8e1d2;
  transition:
    transform 180ms ease,
    background 180ms ease;
}
.book-card__cover:hover {
  transform: translateY(-3px);
  background: #e1d8c6;
}
.book-card__body {
  padding: 18px 4px 0;
}
.book-card__eyebrow {
  margin: 0 0 7px;
  color: var(--color-accent-dark);
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}
.book-card__title {
  display: block;
  overflow: hidden;
  color: var(--color-ink);
  font-family: var(--font-display);
  font-size: 1.24rem;
  font-weight: 650;
  line-height: 1.2;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.book-card__title:hover {
  color: var(--color-brand);
}
.book-card__author {
  margin: 6px 0 16px;
  overflow: hidden;
  color: var(--color-muted);
  font-size: 0.88rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.book-card__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.book-card__footer strong {
  font-size: 0.95rem;
}
.book-card__footer button {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border: 1px solid var(--color-line);
  border-radius: 50%;
  color: var(--color-brand);
  background: transparent;
  cursor: pointer;
  transition: 160ms ease;
}
.book-card__footer button:hover:not(:disabled) {
  border-color: var(--color-brand);
  color: white;
  background: var(--color-brand);
  transform: rotate(90deg);
}
.book-card__footer button:disabled {
  cursor: not-allowed;
  opacity: 0.35;
}
@media (max-width: 600px) {
  .book-card__cover {
    min-height: 245px;
    padding: 22px 16px;
  }
}
</style>
