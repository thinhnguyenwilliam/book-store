<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

import {
  getAdminDashboard,
  type DashboardSnapshot,
} from '@/features/dashboard/api/dashboard.graphql'
import { ApiError } from '@/shared/api/http-client'
import { formatDateTime, formatPrice } from '@/shared/lib/format'
import AppIcon from '@/shared/ui/AppIcon.vue'

const snapshot = ref<DashboardSnapshot>()
const loading = ref(true)
const error = ref('')
const controller = new AbortController()

onMounted(async () => {
  try {
    snapshot.value = await getAdminDashboard(controller.signal)
  } catch (requestError) {
    if (!controller.signal.aborted) {
      error.value =
        requestError instanceof ApiError ? requestError.message : 'Không thể tải dashboard.'
    }
  } finally {
    loading.value = false
  }
})
onBeforeUnmount(() => controller.abort())
</script>

<template>
  <div class="page-stack">
    <section class="welcome-card">
      <div>
        <p class="eyebrow eyebrow--light">Hôm nay tại Book Store</p>
        <h2>Danh mục rõ ràng.<br />Vận hành nhẹ nhàng.</h2>
        <p>Tất cả thay đổi về sách và tồn kho sẽ được phản ánh ngay trên storefront.</p>
      </div>
      <RouterLink class="button button--accent" :to="{ name: 'books' }">
        Quản lý danh mục <AppIcon name="arrow-right" :size="17" />
      </RouterLink>
      <span class="welcome-card__shape welcome-card__shape--one" />
      <span class="welcome-card__shape welcome-card__shape--two" />
    </section>

    <section class="stats-grid" aria-label="Thống kê danh mục">
      <article class="stat-card">
        <span class="stat-card__icon stat-card__icon--green"><AppIcon name="book" /></span>
        <div>
          <p>Sách đã tải</p>
          <strong>{{ snapshot?.loadedCount ?? 0 }}</strong
          ><small v-if="snapshot?.hasMoreBooks">Còn dữ liệu ở trang sau</small
          ><small v-else>Toàn bộ danh mục</small>
        </div>
      </article>
      <article class="stat-card">
        <span class="stat-card__icon stat-card__icon--gold"><AppIcon name="package" /></span>
        <div>
          <p>Tổng tồn kho</p>
          <strong>{{ snapshot?.inventoryUnits ?? 0 }}</strong
          ><small>Đơn vị trong {{ snapshot?.loadedCount ?? 0 }} đầu sách</small>
        </div>
      </article>
      <article class="stat-card">
        <span class="stat-card__icon stat-card__icon--red"><AppIcon name="alert" /></span>
        <div>
          <p>Sắp hết hàng</p>
          <strong>{{ snapshot?.lowStockCount ?? 0 }}</strong
          ><small>Ngưỡng cảnh báo ≤ 5</small>
        </div>
      </article>
      <article class="stat-card">
        <span class="stat-card__icon stat-card__icon--blue"><AppIcon name="value" /></span>
        <div>
          <p>Giá trị tồn kho</p>
          <strong class="stat-card__money">{{
            formatPrice(snapshot?.inventoryValueCents ?? 0)
          }}</strong
          ><small>Tính theo giá bán hiện tại</small>
        </div>
      </article>
    </section>

    <p v-if="error" class="inline-error">{{ error }}</p>

    <section class="dashboard-grid">
      <article class="panel">
        <header class="panel__header">
          <div>
            <p class="eyebrow">Mới cập nhật</p>
            <h2>Sách gần đây</h2>
          </div>
          <RouterLink :to="{ name: 'books' }"
            >Xem tất cả <AppIcon name="arrow-right" :size="15"
          /></RouterLink>
        </header>
        <div v-if="loading" class="skeleton-list"><i v-for="item in 4" :key="item" /></div>
        <div v-else-if="snapshot?.recentBooks.length" class="recent-list">
          <div v-for="book in snapshot.recentBooks" :key="book.id" class="recent-book">
            <span class="book-monogram">{{ book.title.slice(0, 1).toUpperCase() }}</span>
            <div>
              <strong>{{ book.title }}</strong
              ><small>{{ book.author }} · {{ formatDateTime(book.updated_at) }}</small>
            </div>
            <b>{{ formatPrice(book.price_cents) }}</b>
          </div>
        </div>
        <div v-else class="empty-compact">
          <AppIcon name="book" :size="25" />
          <p>Danh mục chưa có sách.</p>
        </div>
      </article>

      <article class="panel">
        <header class="panel__header">
          <div>
            <p class="eyebrow">Cần chú ý</p>
            <h2>Cảnh báo tồn kho</h2>
          </div>
        </header>
        <div v-if="snapshot?.lowStockBooks.length" class="stock-list">
          <div v-for="book in snapshot.lowStockBooks" :key="book.id">
            <span :class="{ 'stock-dot--empty': book.stock === 0 }" class="stock-dot" />
            <p>
              <strong>{{ book.title }}</strong
              ><small>ISBN {{ book.isbn }}</small>
            </p>
            <b>{{ book.stock }}</b>
          </div>
        </div>
        <div v-else class="empty-compact empty-compact--success">
          <AppIcon name="check" :size="22" />
          <p>Không có sách nào sắp hết.</p>
        </div>
      </article>

      <article class="panel panel--wide">
        <header class="panel__header">
          <div>
            <p class="eyebrow">Tài khoản mới</p>
            <h2>Khách hàng gần đây</h2>
          </div>
          <RouterLink :to="{ name: 'customers' }"
            >Quản lý khách hàng <AppIcon name="arrow-right" :size="15"
          /></RouterLink>
        </header>
        <div v-if="loading" class="skeleton-list"><i v-for="item in 3" :key="item" /></div>
        <div v-else-if="snapshot?.customers.length" class="recent-list">
          <div v-for="customer in snapshot.customers" :key="customer.id" class="recent-book">
            <span class="book-monogram">{{ customer.display_name.slice(0, 1).toUpperCase() }}</span>
            <div>
              <strong>{{ customer.display_name }}</strong
              ><small>{{ customer.email }} · {{ formatDateTime(customer.created_at) }}</small>
            </div>
          </div>
        </div>
        <div v-else class="empty-compact">
          <AppIcon name="user" :size="25" />
          <p>Chưa có hồ sơ khách hàng.</p>
        </div>
      </article>
    </section>
  </div>
</template>
