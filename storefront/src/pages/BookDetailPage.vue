<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import { getBookDetail } from '@/features/books/api/book-detail.graphql'
import { trackBookAddedToCart, trackBookViewed } from '@/features/analytics/lib/customer-activity'
import type { Book } from '@/features/books/model/types'
import BookCover from '@/features/books/ui/BookCover.vue'
import CommentThread from '@/features/comments/ui/CommentThread.vue'
import type { Comment } from '@/features/comments/model/types'
import { useCartStore } from '@/features/cart/model/cart.store'
import { useAuthStore } from '@/features/auth/model/auth.store'
import { useNotificationStore } from '@/features/notifications/model/notification.store'
import { ApiError } from '@/shared/api/http-client'
import { formatPrice } from '@/shared/lib/format'
import AppIcon from '@/shared/ui/AppIcon.vue'

const route = useRoute()
const cart = useCartStore()
const auth = useAuthStore()
const notifications = useNotificationStore()
const book = ref<Book>()
const initialComments = ref<Comment[]>([])
const loading = ref(true)
const error = ref('')
let controller: AbortController | undefined

async function loadBook(id: string): Promise<void> {
  controller?.abort()
  controller = new AbortController()
  loading.value = true
  error.value = ''
  try {
    const detail = await getBookDetail(id, controller.signal)
    book.value = detail.book
    initialComments.value = detail.comments
    trackBookViewed(detail.book.id)
  } catch (requestError) {
    if (controller.signal.aborted) return
    error.value =
      requestError instanceof ApiError ? requestError.message : 'Không thể tải cuốn sách.'
  } finally {
    loading.value = false
  }
}

async function addToCart(): Promise<void> {
  if (!book.value) return
  try {
    await cart.add(book.value, auth.isAuthenticated)
    trackBookAddedToCart(book.value.id)
    notifications.show(`Đã thêm “${book.value.title}” vào giỏ.`, 'success')
  } catch {
    notifications.show('Không thể cập nhật giỏ hàng. Vui lòng thử lại.', 'error')
  }
}

watch(
  () => String(route.params.id),
  (id) => loadBook(id),
  { immediate: true },
)
onBeforeUnmount(() => controller?.abort())
</script>

<template>
  <section class="detail-page section">
    <div class="shell">
      <RouterLink to="/sach" class="back-link"
        ><AppIcon name="arrow-left" :size="17" /> Trở lại tủ sách</RouterLink
      >

      <div v-if="loading" class="detail-skeleton">
        <div />
        <section><i /><i /><i /><i /></section>
      </div>
      <div v-else-if="error || !book" class="state-card">
        <AppIcon name="book" :size="36" />
        <h1>Không tìm thấy cuốn sách</h1>
        <p>{{ error || 'Cuốn sách này có thể đã rời khỏi kệ.' }}</p>
        <RouterLink to="/sach" class="button button--primary">Về tủ sách</RouterLink>
      </div>
      <div v-else class="detail-grid">
        <div class="detail-cover-wrap">
          <BookCover :title="book.title" :author="book.author" :isbn="book.isbn" size="large" />
        </div>
        <div class="detail-copy">
          <p class="eyebrow">Sách tuyển chọn · ISBN {{ book.isbn }}</p>
          <h1>{{ book.title }}</h1>
          <p class="detail-author">
            bởi <strong>{{ book.author }}</strong>
          </p>
          <div class="detail-rating"><span>★★★★★</span><small>4.9 · Độc giả yêu thích</small></div>
          <p class="detail-description">
            Một cuốn sách dành cho những ai muốn nhìn lại cách mình suy nghĩ, xây dựng và tạo nên
            những điều có giá trị lâu dài. Mộc Thư gửi đến bạn bản tuyển chọn với sự trân trọng.
          </p>
          <div class="detail-price-row">
            <strong>{{ formatPrice(book.price_cents) }}</strong>
            <span :class="{ 'is-out': book.stock < 1 }">{{
              book.stock > 0 ? `Còn ${book.stock} cuốn` : 'Tạm hết hàng'
            }}</span>
          </div>
          <button
            class="button button--primary detail-add"
            type="button"
            :disabled="book.stock < 1"
            @click="addToCart"
          >
            <AppIcon name="bag" :size="19" /> Thêm vào giỏ
          </button>
          <div class="detail-perks">
            <div>
              <AppIcon name="check" /><span
                ><b>Đóng gói cẩn thận</b><small>Không nhựa dùng một lần</small></span
              >
            </div>
            <div>
              <AppIcon name="check" /><span
                ><b>Đổi trả trong 7 ngày</b><small>Nếu sách có lỗi in ấn</small></span
              >
            </div>
          </div>
        </div>
      </div>
      <CommentThread
        v-if="book"
        :key="book.id"
        :book-id="book.id"
        :initial-comments="initialComments"
      />
    </div>
  </section>
</template>

<style scoped>
.back-link {
  display: inline-flex;
  gap: 6px;
  align-items: center;
  margin-bottom: 42px;
  color: var(--color-muted);
  font-size: 0.82rem;
  font-weight: 700;
}
.back-link:hover {
  color: var(--color-brand);
}
.detail-grid {
  display: grid;
  grid-template-columns: minmax(360px, 0.9fr) 1.1fr;
  gap: 90px;
  align-items: center;
}
.detail-cover-wrap {
  display: grid;
  min-height: 610px;
  place-items: center;
  border-radius: var(--radius-xl);
  background: #ddd6c6;
  background-image: radial-gradient(circle at 75% 18%, rgb(255 255 255 / 42%), transparent 28%);
}
.detail-copy h1 {
  max-width: 680px;
  margin: 14px 0 14px;
  font-family: var(--font-display);
  font-size: clamp(2.8rem, 5vw, 5.2rem);
  font-weight: 550;
  letter-spacing: -0.05em;
  line-height: 0.95;
}
.detail-author {
  margin: 0;
  color: var(--color-muted);
}
.detail-author strong {
  color: var(--color-ink);
  font-weight: 650;
}
.detail-rating {
  display: flex;
  gap: 12px;
  align-items: center;
  margin: 24px 0;
}
.detail-rating span {
  color: #d89a33;
  letter-spacing: 0.08em;
}
.detail-rating small {
  color: var(--color-muted);
}
.detail-description {
  max-width: 620px;
  padding: 24px 0;
  border-top: 1px solid var(--color-line);
  border-bottom: 1px solid var(--color-line);
  color: var(--color-muted);
  line-height: 1.8;
}
.detail-price-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  max-width: 620px;
  margin: 24px 0;
}
.detail-price-row strong {
  color: var(--color-brand);
  font-family: var(--font-display);
  font-size: 2rem;
}
.detail-price-row span {
  padding: 7px 10px;
  border-radius: 20px;
  color: #29714e;
  background: #e4f1e8;
  font-size: 0.72rem;
  font-weight: 750;
}
.detail-price-row span.is-out {
  color: #8b352e;
  background: #f4e3df;
}
.detail-add {
  width: min(100%, 620px);
  justify-content: center;
}
.detail-perks {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 22px;
  max-width: 620px;
  margin-top: 28px;
}
.detail-perks > div {
  display: flex;
  gap: 10px;
  align-items: flex-start;
}
.detail-perks svg {
  flex: 0 0 auto;
  color: var(--color-accent-dark);
}
.detail-perks span {
  display: flex;
  flex-direction: column;
  gap: 3px;
}
.detail-perks b {
  font-size: 0.8rem;
}
.detail-perks small {
  color: var(--color-muted);
  font-size: 0.7rem;
}
.detail-skeleton {
  display: grid;
  grid-template-columns: 0.9fr 1.1fr;
  gap: 90px;
}
.detail-skeleton > div {
  min-height: 610px;
  border-radius: var(--radius-xl);
  background: #e7e1d5;
}
.detail-skeleton section {
  padding-top: 80px;
}
.detail-skeleton i {
  display: block;
  width: 80%;
  height: 22px;
  margin: 18px 0;
  border-radius: 8px;
  background: #e7e1d5;
}
.detail-skeleton i:nth-child(2) {
  width: 95%;
  height: 80px;
}
.state-card {
  display: grid;
  min-height: 420px;
  place-items: center;
  align-content: center;
  text-align: center;
}
.state-card h1 {
  margin: 18px 0 8px;
  font-family: var(--font-display);
  font-size: 2.4rem;
}
.state-card p {
  margin: 0 0 24px;
  color: var(--color-muted);
}
@media (max-width: 850px) {
  .detail-grid,
  .detail-skeleton {
    grid-template-columns: 1fr;
    gap: 46px;
  }
  .detail-cover-wrap {
    min-height: 500px;
  }
}
@media (max-width: 520px) {
  .detail-cover-wrap {
    min-height: 420px;
  }
  .detail-perks {
    grid-template-columns: 1fr;
  }
}
</style>
