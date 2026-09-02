<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'

import { getPayment } from '@/features/payments/api/payment.api'
import type { Payment } from '@/features/payments/model/types'
import { useCartStore } from '@/features/cart/model/cart.store'
import { clearCheckoutAttempt } from '@/features/orders/lib/checkout-attempt'
import { ApiError } from '@/shared/api/http-client'
import { formatPrice } from '@/shared/lib/format'

const route = useRoute()
const cart = useCartStore()
const payment = ref<Payment | null>(null)
const loading = ref(true)
const error = ref('')
let stopped = false

const paymentId = computed(() => {
  const value = route.query.vnp_TxnRef
  return typeof value === 'string' ? value.trim() : ''
})
const title = computed(() => {
  switch (payment.value?.status) {
    case 'succeeded':
      return 'Thanh toán thành công'
    case 'failed':
      return 'Thanh toán chưa thành công'
    case 'refunded':
      return 'Khoản thanh toán đã được hoàn'
    case 'refund_pending':
      return 'Yêu cầu hoàn tiền đang xử lý'
    default:
      return 'Đang xác nhận thanh toán'
  }
})

function delay(duration: number): Promise<void> {
  return new Promise((resolve) => window.setTimeout(resolve, duration))
}

async function loadPayment(): Promise<void> {
  if (!paymentId.value) {
    error.value = 'Thiếu mã tham chiếu thanh toán.'
    loading.value = false
    return
  }
  for (let attempt = 0; attempt < 8 && !stopped; attempt += 1) {
    try {
      payment.value = await getPayment(paymentId.value)
      error.value = ''
      if (payment.value.status !== 'pending') {
        clearCheckoutAttempt()
        await cart.syncAuthenticated(true)
        break
      }
    } catch (requestError) {
      error.value =
        requestError instanceof ApiError
          ? requestError.message
          : 'Không thể kiểm tra trạng thái thanh toán.'
      break
    }
    await delay(2000)
  }
  loading.value = false
}

onMounted(loadPayment)
onBeforeUnmount(() => {
  stopped = true
})
</script>

<template>
  <section class="section payment-result">
    <div class="shell payment-result__card" aria-live="polite">
      <p class="eyebrow">VNPAY</p>
      <h1>{{ title }}</h1>
      <p v-if="loading">Hệ thống đang chờ IPN và kiểm tra trạng thái từ backend…</p>
      <p v-else-if="error" class="form-error">{{ error }}</p>
      <template v-else-if="payment">
        <p>
          Trạng thái hiển thị lấy từ Payment Service, không lấy từ tham số callback trên trình
          duyệt.
        </p>
        <dl>
          <div>
            <dt>Mã thanh toán</dt>
            <dd>{{ payment.id }}</dd>
          </div>
          <div>
            <dt>Mã đơn hàng</dt>
            <dd>{{ payment.order_id }}</dd>
          </div>
          <div>
            <dt>Số tiền</dt>
            <dd>{{ formatPrice(payment.amount_cents) }}</dd>
          </div>
          <div>
            <dt>Trạng thái</dt>
            <dd>{{ payment.status }}</dd>
          </div>
        </dl>
      </template>
      <div class="payment-result__actions">
        <RouterLink
          v-if="payment"
          class="button button--outline"
          :to="{ name: 'order-detail', params: { id: payment.order_id } }"
          >Xem đơn hàng</RouterLink
        >
        <RouterLink class="button button--primary" to="/tai-khoan">Về tài khoản</RouterLink>
        <RouterLink class="button button--outline" to="/sach">Tiếp tục xem sách</RouterLink>
      </div>
    </div>
  </section>
</template>

<style scoped>
.payment-result {
  min-height: 65vh;
  background: var(--color-paper);
}
.payment-result__card {
  max-width: 760px;
  padding: clamp(32px, 6vw, 64px);
  border: 1px solid var(--color-line);
  border-radius: var(--radius-lg);
  background: white;
  box-shadow: var(--shadow-md);
}
.payment-result h1 {
  margin: 8px 0 18px;
  font-family: var(--font-display);
  font-size: clamp(2rem, 5vw, 3.5rem);
}
.payment-result dl {
  margin: 28px 0;
}
.payment-result dl div {
  display: grid;
  grid-template-columns: minmax(120px, 0.4fr) 1fr;
  gap: 20px;
  padding: 12px 0;
  border-bottom: 1px solid var(--color-line);
}
.payment-result dt {
  color: var(--color-muted);
}
.payment-result dd {
  margin: 0;
  overflow-wrap: anywhere;
  font-weight: 700;
}
.payment-result__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 28px;
}
</style>
