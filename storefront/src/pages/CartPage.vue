<script setup lang="ts">
import { useCartStore } from '@/features/cart/model/cart.store'
import BookCover from '@/features/books/ui/BookCover.vue'
import { useNotificationStore } from '@/features/notifications/model/notification.store'
import { formatPrice } from '@/shared/lib/format'
import AppIcon from '@/shared/ui/AppIcon.vue'

const cart = useCartStore()
const notifications = useNotificationStore()

function checkout(): void {
  notifications.show(
    'Backend checkout chưa được triển khai. Giỏ hàng của bạn vẫn được giữ lại.',
    'info',
  )
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
              @click="cart.setQuantity(item.book.id, item.quantity - 1)"
            >
              <AppIcon name="minus" :size="15" />
            </button>
            <span>{{ item.quantity }}</span>
            <button
              type="button"
              aria-label="Tăng số lượng"
              :disabled="item.quantity >= item.book.stock"
              @click="cart.setQuantity(item.book.id, item.quantity + 1)"
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
            @click="cart.remove(item.book.id)"
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
        <button class="button button--accent" type="button" @click="checkout">
          Tiến hành đặt hàng
        </button>
        <small>Thanh toán chưa khả dụng vì backend hiện chưa có Order Service.</small>
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
