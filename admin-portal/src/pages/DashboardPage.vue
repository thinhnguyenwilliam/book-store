<script setup lang="ts">
import { computed, onMounted } from 'vue'

import { useAdminBooksStore } from '@/features/books/model/books.store'
import { formatDateTime, formatPrice } from '@/shared/lib/format'
import AppIcon from '@/shared/ui/AppIcon.vue'

const books = useAdminBooksStore()
const recentBooks = computed(() => books.books.slice(0, 5))
const lowStockBooks = computed(() =>
  [...books.books]
    .filter((book) => book.stock <= 5)
    .sort((a, b) => a.stock - b.stock)
    .slice(0, 5),
)

onMounted(() => books.fetchInitial())
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
          <strong>{{ books.totalLoaded }}</strong
          ><small v-if="books.hasMore">Còn dữ liệu ở trang sau</small
          ><small v-else>Toàn bộ danh mục</small>
        </div>
      </article>
      <article class="stat-card">
        <span class="stat-card__icon stat-card__icon--gold"><AppIcon name="package" /></span>
        <div>
          <p>Tổng tồn kho</p>
          <strong>{{ books.inventoryUnits }}</strong
          ><small>Đơn vị trong {{ books.totalLoaded }} đầu sách</small>
        </div>
      </article>
      <article class="stat-card">
        <span class="stat-card__icon stat-card__icon--red"><AppIcon name="alert" /></span>
        <div>
          <p>Sắp hết hàng</p>
          <strong>{{ books.lowStockCount }}</strong
          ><small>Ngưỡng cảnh báo ≤ 5</small>
        </div>
      </article>
      <article class="stat-card">
        <span class="stat-card__icon stat-card__icon--blue"><AppIcon name="value" /></span>
        <div>
          <p>Giá trị tồn kho</p>
          <strong class="stat-card__money">{{ formatPrice(books.inventoryValueCents) }}</strong
          ><small>Tính theo giá bán hiện tại</small>
        </div>
      </article>
    </section>

    <p v-if="books.error" class="inline-error">{{ books.error }}</p>

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
        <div v-if="books.loading" class="skeleton-list"><i v-for="item in 4" :key="item" /></div>
        <div v-else-if="recentBooks.length" class="recent-list">
          <div v-for="book in recentBooks" :key="book.id" class="recent-book">
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
        <div v-if="lowStockBooks.length" class="stock-list">
          <div v-for="book in lowStockBooks" :key="book.id">
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
    </section>
  </div>
</template>
