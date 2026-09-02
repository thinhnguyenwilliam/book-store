<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { useCartStore } from '@/features/cart/model/cart.store'
import { useAuthStore } from '@/features/auth/model/auth.store'
import {
  trackBookRemovedFromCart,
  trackCheckoutStarted,
} from '@/features/analytics/lib/customer-activity'
import { createOrder, payOrder } from '@/features/orders/api/checkout.api'
import {
  clearCheckoutAttempt,
  getCheckoutAttempt,
  saveCheckoutOrder,
} from '@/features/orders/lib/checkout-attempt'
import BookCover from '@/features/books/ui/BookCover.vue'
import { useNotificationStore } from '@/features/notifications/model/notification.store'
import { formatPrice } from '@/shared/lib/format'
import AppIcon from '@/shared/ui/AppIcon.vue'
import { ApiError } from '@/shared/api/http-client'
import { env } from '@/shared/config/env'

const cart = useCartStore()
const auth = useAuthStore()
const notifications = useNotificationStore()
const route = useRoute()
const router = useRouter()
const provider = ref<'wallet' | 'vnpay'>('wallet')
const processing = ref(false)
const checkoutError = ref('')

onMounted(() => {
  if (auth.isAuthenticated) void cart.syncAuthenticated().catch(() => undefined)
})

async function checkout(): Promise<void> {
  if (!auth.isAuthenticated) {
    await router.push({ name: 'login', query: { redirect: route.fullPath } })
    return
  }
  if (processing.value || cart.isEmpty) return
  trackCheckoutStarted()
  processing.value = true
  checkoutError.value = ''
  try {
    await cart.syncAuthenticated()
    let attempt = getCheckoutAttempt()
    const order = await createOrder(attempt.orderKey)
    attempt = saveCheckoutOrder(attempt, order.id)
    const payment = await payOrder(order.id, provider.value, attempt.paymentKey)
    if (payment.status === 'pending' && payment.checkout_url) {
      window.location.assign(payment.checkout_url)
      return
    }
    if (payment.status !== 'succeeded') {
      throw new Error('Thanh toán chưa được xác nhận.')
    }
    clearCheckoutAttempt()
    await cart.syncAuthenticated(true)
    notifications.show('Đơn hàng đã được thanh toán thành công.', 'success')
    await router.push({ name: 'order-detail', params: { id: order.id } })
  } catch (requestError) {
    if (requestError instanceof ApiError && [400, 409, 412].includes(requestError.status)) {
      clearCheckoutAttempt()
      await cart.syncAuthenticated(true).catch(() => undefined)
    }
    checkoutError.value =
      requestError instanceof ApiError
        ? requestError.message
        : requestError instanceof Error
          ? requestError.message
          : 'Không thể hoàn tất checkout. Vui lòng thử lại.'
  } finally {
    processing.value = false
  }
}

async function setQuantity(bookID: string, current: number, next: number): Promise<void> {
  if (next <= 0) trackBookRemovedFromCart(bookID, current)
  try {
    await cart.setQuantity(bookID, next, auth.isAuthenticated)
  } catch {
    notifications.show('Không thể cập nhật số lượng.', 'error')
  }
}

async function remove(bookID: string, quantity: number): Promise<void> {
  trackBookRemovedFromCart(bookID, quantity)
  try {
    await cart.remove(bookID, auth.isAuthenticated)
  } catch {
    notifications.show('Không thể xóa sách khỏi giỏ.', 'error')
  }
}
</script>

<template>
  <section class="page-hero page-hero--compact">
    <div class="shell">
      <p class="eyebrow">Đơn hàng của bạn</p>
      <h1>Giỏ sách</h1>
    </div>
  </section>

  <section class="section cart-page">
    <div v-if="cart.isEmpty" class="shell empty-cart">
      <span class="empty-cart__icon"><AppIcon name="bag" :size="36" /></span>
      <h2>Giỏ sách vẫn còn trống</h2>
      <p>Có lẽ một cuốn sách hay đang chờ bạn ở tủ sách.</p>
      <RouterLink to="/sach" class="button button--primary">Khám phá tủ sách</RouterLink>
    </div>

    <div v-else class="shell cart-grid">
      <div class="cart-items">
        <article v-for="item in cart.items" :key="item.book.id" class="cart-item">
          <RouterLink :to="`/sach/${item.book.id}`"
            ><BookCover
              :title="item.book.title"
              :author="item.book.author"
              :isbn="item.book.isbn"
              size="small"
          /></RouterLink>
          <div class="cart-item__copy">
            <RouterLink :to="`/sach/${item.book.id}`"
              ><h2>{{ item.book.title }}</h2></RouterLink
            >
            <p>{{ item.book.author }}</p>
            <strong>{{ formatPrice(item.book.price_cents) }}</strong>
          </div>
          <div class="quantity" aria-label="Số lượng">
            <button
              type="button"
              aria-label="Giảm số lượng"
              @click="setQuantity(item.book.id, item.quantity, item.quantity - 1)"
            >
              <AppIcon name="minus" :size="15" />
            </button>
            <span>{{ item.quantity }}</span>
            <button
              type="button"
              aria-label="Tăng số lượng"
              :disabled="item.quantity >= item.book.stock"
              @click="setQuantity(item.book.id, item.quantity, item.quantity + 1)"
            >
              <AppIcon name="plus" :size="15" />
            </button>
          </div>
          <strong class="cart-item__total">{{
            formatPrice(item.book.price_cents * item.quantity)
          }}</strong>
          <button
            class="remove-button"
            type="button"
            aria-label="Xóa khỏi giỏ"
            @click="remove(item.book.id, item.quantity)"
          >
            <AppIcon name="trash" :size="18" />
          </button>
        </article>
      </div>

      <aside class="order-summary">
        <p class="eyebrow">Tóm tắt đơn hàng</p>
        <h2>{{ cart.itemCount }} cuốn sách</h2>
        <dl>
          <div>
            <dt>Tạm tính</dt>
            <dd>{{ formatPrice(cart.subtotalCents) }}</dd>
          </div>
          <div>
            <dt>Phí giao hàng</dt>
            <dd>Miễn phí</dd>
          </div>
          <div class="order-summary__total">
            <dt>Tổng cộng</dt>
            <dd>{{ formatPrice(cart.subtotalCents) }}</dd>
          </div>
        </dl>
        <fieldset class="payment-provider" :disabled="processing">
          <legend>Phương thức thanh toán</legend>
          <label><input v-model="provider" type="radio" value="wallet" /> Ví Book Store</label>
          <label v-if="env.vnpayEnabled"
            ><input v-model="provider" type="radio" value="vnpay" /> VNPAY</label
          >
        </fieldset>
        <p v-if="checkoutError" class="form-error" role="alert">{{ checkoutError }}</p>
        <button
          class="button button--accent"
          type="button"
          :disabled="processing || cart.loading"
          @click="checkout"
        >
          {{ processing ? 'Đang giữ hàng và thanh toán…' : 'Tiến hành đặt hàng' }}
        </button>
        <small>Hàng được giữ tối đa 15 phút. Giá và tồn kho sẽ được backend xác nhận lại.</small>
      </aside>
    </div>
  </section>
</template>

<style scoped>
.page-hero--compact {
  padding: 65px 0 54px;
  background: #e8e1d2;
}
.page-hero h1 {
  margin: 8px 0 0;
  font-family: var(--font-display);
  font-size: clamp(3rem, 6vw, 5rem);
  font-weight: 550;
  letter-spacing: -0.05em;
}
.empty-cart {
  display: grid;
  min-height: 430px;
  place-items: center;
  align-content: center;
  text-align: center;
}
.empty-cart__icon {
  display: grid;
  width: 76px;
  height: 76px;
  place-items: center;
  border-radius: 50%;
  color: var(--color-brand);
  background: var(--color-surface);
}
.empty-cart h2 {
  margin: 20px 0 8px;
  font-family: var(--font-display);
  font-size: 2rem;
}
.empty-cart p {
  margin: 0 0 24px;
  color: var(--color-muted);
}
.cart-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 360px;
  gap: 60px;
  align-items: start;
}
.cart-items {
  border-top: 1px solid var(--color-line);
}
.cart-item {
  display: grid;
  grid-template-columns: 80px minmax(0, 1fr) auto auto auto;
  gap: 22px;
  align-items: center;
  padding: 24px 0;
  border-bottom: 1px solid var(--color-line);
}
.cart-item__copy h2 {
  margin: 0 0 5px;
  font-family: var(--font-display);
  font-size: 1.18rem;
}
.cart-item__copy p {
  margin: 0 0 8px;
  color: var(--color-muted);
  font-size: 0.8rem;
}
.cart-item__copy strong {
  display: none;
  font-size: 0.82rem;
}
.quantity {
  display: flex;
  align-items: center;
  border: 1px solid var(--color-line);
  border-radius: 30px;
}
.quantity button {
  display: grid;
  width: 32px;
  height: 32px;
  place-items: center;
  border: 0;
  border-radius: 50%;
  background: transparent;
  cursor: pointer;
}
.quantity button:disabled {
  opacity: 0.25;
}
.quantity span {
  min-width: 28px;
  text-align: center;
  font-size: 0.8rem;
  font-weight: 750;
}
.cart-item__total {
  min-width: 90px;
  font-size: 0.9rem;
  text-align: right;
}
.remove-button {
  display: grid;
  width: 36px;
  height: 36px;
  place-items: center;
  border: 0;
  color: var(--color-muted);
  background: transparent;
  cursor: pointer;
}
.remove-button:hover {
  color: var(--color-danger);
}
.order-summary {
  position: sticky;
  top: 104px;
  padding: 34px;
  border-radius: var(--radius-lg);
  color: white;
  background: var(--color-brand);
}
.order-summary .eyebrow {
  color: var(--color-accent);
}
.order-summary h2 {
  margin: 8px 0 28px;
  font-family: var(--font-display);
  font-size: 2rem;
}
.order-summary dl {
  display: grid;
  gap: 15px;
  margin: 0 0 26px;
}
.order-summary dl div {
  display: flex;
  justify-content: space-between;
  color: rgb(255 255 255 / 70%);
  font-size: 0.82rem;
}
.order-summary dd {
  margin: 0;
  color: white;
  font-weight: 700;
}
.order-summary__total {
  margin-top: 10px;
  padding-top: 20px;
  border-top: 1px solid rgb(255 255 255 / 18%);
  font-size: 1rem !important;
}
.payment-provider {
  display: grid;
  gap: 9px;
  margin: 22px 0;
  padding: 0;
  border: 0;
}
.payment-provider legend {
  margin-bottom: 10px;
  font-weight: 800;
}
.payment-provider label {
  display: flex;
  gap: 9px;
  align-items: center;
  color: var(--color-muted);
}
.order-summary__total dd {
  font-family: var(--font-display);
  font-size: 1.45rem;
}
.order-summary .button {
  width: 100%;
  justify-content: center;
}
.order-summary small {
  display: block;
  margin-top: 14px;
  color: rgb(255 255 255 / 55%);
  font-size: 0.67rem;
  line-height: 1.5;
  text-align: center;
}
@media (max-width: 900px) {
  .cart-grid {
    grid-template-columns: 1fr;
  }
  .order-summary {
    position: static;
  }
}
@media (max-width: 620px) {
  .cart-item {
    grid-template-columns: 70px 1fr auto;
    gap: 14px;
  }
  .cart-item__copy strong {
    display: block;
  }
  .quantity {
    grid-column: 2;
    justify-self: start;
  }
  .cart-item__total {
    display: none;
  }
  .remove-button {
    grid-row: 1;
    grid-column: 3;
  }
}
</style>
