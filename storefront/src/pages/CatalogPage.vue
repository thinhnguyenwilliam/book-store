<script setup lang="ts">
import { onMounted } from 'vue'

import { useBooksStore } from '@/features/books/model/books.store'
import BookCard from '@/features/books/ui/BookCard.vue'
import AppIcon from '@/shared/ui/AppIcon.vue'

const booksStore = useBooksStore()
onMounted(() => booksStore.fetchInitial())
</script>

<template>
  <section class="page-hero page-hero--catalog">
    <div class="shell page-hero__inner">
      <p class="eyebrow">Tủ sách Mộc Thư</p>
      <h1>Tìm cuốn sách<br />dành cho lúc này.</h1>
      <p>Mỗi cuốn sách là một cánh cửa. Bạn chỉ cần chọn cánh cửa muốn mở.</p>
    </div>
  </section>

  <section class="section catalog">
    <div class="shell">
      <div class="catalog__bar">
        <div>
          <span>{{ booksStore.books.length }} tựa sách đã tải</span>
          <small> Sắp xếp theo sách mới nhất</small>
        </div>
        <span class="catalog__live"><i /> Dữ liệu trực tiếp từ Book Service</span>
      </div>

      <div v-if="booksStore.loading" class="catalog-grid" aria-label="Đang tải sách">
        <div v-for="item in 8" :key="item" class="catalog-skeleton" />
      </div>
      <div v-else-if="booksStore.isEmpty && booksStore.error" class="state-card">
        <AppIcon name="book" :size="34" />
        <h2>Chưa thể mở tủ sách</h2>
        <p>{{ booksStore.error }}</p>
        <button class="button button--primary" type="button" @click="booksStore.fetchInitial(true)">
          Thử lại
        </button>
      </div>
      <div v-else-if="booksStore.isEmpty" class="state-card">
        <AppIcon name="book" :size="34" />
        <h2>Tủ sách đang trống</h2>
        <p>Những cuốn sách đầu tiên đang trên đường đến kệ.</p>
      </div>
      <template v-else>
        <div class="catalog-grid">
          <BookCard v-for="book in booksStore.books" :key="book.id" :book="book" />
        </div>
        <div class="load-more">
          <p v-if="booksStore.error" class="load-more__error">{{ booksStore.error }}</p>
          <button
            v-if="booksStore.hasMore"
            class="button button--outline"
            type="button"
            :disabled="booksStore.loadingMore"
            @click="booksStore.fetchNext"
          >
            {{ booksStore.loadingMore ? 'Đang mở thêm kệ…' : 'Xem thêm sách' }}
            <AppIcon v-if="!booksStore.loadingMore" name="arrow-right" :size="18" />
          </button>
          <p v-else class="load-more__end"><span /> Bạn đã xem hết tủ sách <span /></p>
        </div>
      </template>
    </div>
  </section>
</template>

<style scoped>
.page-hero--catalog {
  background: #dfe8dc;
}
.page-hero__inner {
  position: relative;
  padding-top: 86px;
  padding-bottom: 72px;
}
.page-hero__inner::after {
  position: absolute;
  right: 5%;
  bottom: 0;
  width: 180px;
  height: 86%;
  border-radius: 100% 0 0;
  background: #c7d4bd;
  content: '';
  opacity: 0.75;
}
.page-hero h1 {
  position: relative;
  z-index: 1;
  max-width: 700px;
  margin: 14px 0 18px;
  font-family: var(--font-display);
  font-size: clamp(3rem, 6vw, 5.4rem);
  font-weight: 550;
  letter-spacing: -0.05em;
  line-height: 0.94;
}
.page-hero p:last-child {
  position: relative;
  z-index: 1;
  max-width: 500px;
  margin: 0;
  color: var(--color-muted);
  line-height: 1.7;
}
.catalog__bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 34px;
  padding-bottom: 18px;
  border-bottom: 1px solid var(--color-line);
  color: var(--color-muted);
  font-size: 0.8rem;
}
.catalog__bar small {
  margin-left: 10px;
  padding-left: 12px;
  border-left: 1px solid var(--color-line);
}
.catalog__live {
  display: inline-flex;
  gap: 7px;
  align-items: center;
}
.catalog__live i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #43a66e;
  box-shadow: 0 0 0 4px rgb(67 166 110 / 13%);
}
.catalog-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 28px;
  row-gap: 58px;
}
.catalog-skeleton {
  min-height: 430px;
  border-radius: var(--radius-lg);
  background: linear-gradient(90deg, #e8e2d7 25%, #f5f1e9 50%, #e8e2d7 75%);
  background-size: 300% 100%;
  animation: shimmer 1.5s infinite;
}
.load-more {
  display: grid;
  place-items: center;
  margin-top: 72px;
}
.load-more__error {
  margin: 0 0 16px;
  color: var(--color-danger);
}
.load-more__end {
  display: flex;
  gap: 14px;
  align-items: center;
  color: var(--color-muted);
  font-family: var(--font-display);
  font-size: 0.88rem;
  font-style: italic;
}
.load-more__end span {
  width: 48px;
  height: 1px;
  background: var(--color-line);
}
.state-card {
  display: grid;
  max-width: 560px;
  min-height: 310px;
  place-items: center;
  margin: 0 auto;
  padding: 48px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-lg);
  text-align: center;
}
.state-card svg {
  color: var(--color-accent-dark);
}
.state-card h2 {
  margin: 16px 0 6px;
  font-family: var(--font-display);
  font-size: 2rem;
}
.state-card p {
  margin: 0 0 24px;
  color: var(--color-muted);
}
@keyframes shimmer {
  to {
    background-position: -300% 0;
  }
}
@media (max-width: 980px) {
  .catalog-grid {
    grid-template-columns: repeat(3, 1fr);
  }
}
@media (max-width: 720px) {
  .catalog-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 16px;
    row-gap: 42px;
  }
  .catalog__live {
    display: none;
  }
  .page-hero__inner::after {
    width: 100px;
  }
}
@media (max-width: 430px) {
  .catalog__bar small {
    display: none;
  }
}
</style>
