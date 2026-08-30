<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'

import { getOrderDetail, type OrderDetail } from '@/features/orders/api/order-detail.graphql'
import { ApiError } from '@/shared/api/http-client'
import { formatPrice } from '@/shared/lib/format'

const route = useRoute()
const detail = ref<OrderDetail>()
const loading = ref(true)
const error = ref('')
const controller = new AbortController()

onMounted(async () => {
  try {
    detail.value = await getOrderDetail(String(route.params.id), controller.signal)
  } catch (requestError) {
    if (!controller.signal.aborted) {
      error.value =
        requestError instanceof ApiError ? requestError.message : 'Không thể tải đơn hàng.'
    }
  } finally {
    loading.value = false
  }
})
onBeforeUnmount(() => controller.abort())
</script>

<template>
  <section class="section order-detail-page">
    <div class="shell order-detail-card">
      <RouterLink to="/tai-khoan" class="order-detail-back">← Quay lại tài khoản</RouterLink>
      <p class="eyebrow">Chi tiết đơn hàng</p>
      <p v-if="loading">Đang tải đơn hàng…</p>
      <p v-else-if="error" class="form-error">{{ error }}</p>
      <template v-else-if="detail">
        <header>
          <div>
            <h1>Đơn #{{ detail.order.id.slice(0, 8) }}</h1>
            <small>{{ new Date(detail.order.createdAt).toLocaleString('vi-VN') }}</small>
          </div>
          <span>{{ detail.order.status }}</span>
        </header>

        <div class="order-items">
          <article v-for="item in detail.order.items" :key="item.id">
            <div>
              <strong>{{ item.title }}</strong>
              <small>{{ item.quantity }} × {{ formatPrice(item.unitPriceCents) }}</small>
            </div>
            <b>{{ formatPrice(item.subtotalCents) }}</b>
          </article>
        </div>

        <dl>
          <div>
            <dt>Tổng thanh toán</dt>
            <dd>{{ formatPrice(detail.order.totalCents) }}</dd>
          </div>
          <div>
            <dt>Trạng thái payment</dt>
            <dd>{{ detail.payment?.status || 'Chưa thanh toán' }}</dd>
          </div>
          <div v-if="detail.payment">
            <dt>Nhà cung cấp</dt>
            <dd>{{ detail.payment.provider }}</dd>
          </div>
          <div v-if="detail.order.failureReason">
            <dt>Lý do lỗi</dt>
            <dd>{{ detail.order.failureReason }}</dd>
          </div>
        </dl>
      </template>
    </div>
  </section>
</template>

<style scoped>
.order-detail-page {
  min-height: 65vh;
}
.order-detail-card {
  max-width: 850px;
  padding: clamp(28px, 5vw, 56px);
  border: 1px solid var(--color-line);
  border-radius: var(--radius-lg);
  background: white;
}
.order-detail-back {
  display: inline-block;
  margin-bottom: 28px;
  color: var(--color-muted);
  font-weight: 700;
}
header {
  display: flex;
  justify-content: space-between;
  gap: 20px;
  align-items: start;
  margin: 10px 0 30px;
}
h1 {
  margin: 0 0 6px;
  font-family: var(--font-display);
  font-size: clamp(2rem, 5vw, 3.3rem);
}
header small {
  color: var(--color-muted);
}
header span {
  padding: 8px 12px;
  border-radius: 999px;
  color: var(--color-brand);
  background: #e4f1e8;
  font-size: 0.75rem;
  font-weight: 800;
}
.order-items {
  display: grid;
}
.order-items article {
  display: flex;
  justify-content: space-between;
  gap: 20px;
  padding: 18px 0;
  border-bottom: 1px solid var(--color-line);
}
.order-items article div {
  display: grid;
  gap: 5px;
}
.order-items small,
dt {
  color: var(--color-muted);
}
dl {
  margin-top: 30px;
}
dl div {
  display: grid;
  grid-template-columns: minmax(150px, 0.5fr) 1fr;
  gap: 20px;
  padding: 10px 0;
}
dd {
  margin: 0;
  font-weight: 750;
  overflow-wrap: anywhere;
}
</style>
