<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

import { trackBookSearched } from '@/features/analytics/lib/customer-activity'
import {
  suggestBooks,
  type BookSearchSort,
  type SearchBooksParams,
} from '@/features/books/api/books.api'
import { useBooksStore } from '@/features/books/model/books.store'
import type { BookSearchHit } from '@/features/books/model/types'
import BookCard from '@/features/books/ui/BookCard.vue'
import AppIcon from '@/shared/ui/AppIcon.vue'

const booksStore = useBooksStore()
const query = ref('')
const minPrice = ref<number>()
const maxPrice = ref<number>()
const stock = ref<'all' | 'in' | 'out'>('all')
const sort = ref<BookSearchSort>('relevance')
const suggestions = ref<BookSearchHit[]>([])
const suggestionsOpen = ref(false)
const suggesting = ref(false)
let suggestionTimer: number | undefined
let suggestionRequest: AbortController | undefined

const resultLabel = computed(() => {
  if (booksStore.searchTotal !== undefined) {
    return `${booksStore.searchTotal} kết quả trong ${booksStore.searchTookMS ?? 0} ms`
  }
  return `${booksStore.books.length} tựa sách đã tải`
})

onMounted(() => booksStore.fetchInitial())
onBeforeUnmount(() => {
  window.clearTimeout(suggestionTimer)
  suggestionRequest?.abort()
})

watch(query, (value) => {
  window.clearTimeout(suggestionTimer)
  suggestionRequest?.abort()
  const normalized = value.trim()
  if (normalized.length < 2) {
    suggestions.value = []
    suggestionsOpen.value = false
    return
  }
  suggestionTimer = window.setTimeout(async () => {
    suggestionRequest = new AbortController()
    suggesting.value = true
    try {
      const response = await suggestBooks(normalized, suggestionRequest.signal)
      suggestions.value = response.data
      suggestionsOpen.value = true
    } catch {
      if (!suggestionRequest.signal.aborted) suggestions.value = []
    } finally {
      suggesting.value = false
    }
  }, 250)
})

function searchParams(): Omit<SearchBooksParams, 'cursor' | 'limit' | 'signal'> {
  const normalizedQuery = query.value.trim()
  return {
    ...(normalizedQuery ? { query: normalizedQuery } : {}),
    ...(minPrice.value !== undefined ? { minPriceCents: minPrice.value } : {}),
    ...(maxPrice.value !== undefined ? { maxPriceCents: maxPrice.value } : {}),
    ...(stock.value !== 'all' ? { inStock: stock.value === 'in' } : {}),
    sort: sort.value,
  }
}

async function submitSearch(): Promise<void> {
  suggestionsOpen.value = false
  if (query.value.trim().length >= 2) trackBookSearched(query.value)
  await booksStore.fetchSearch(searchParams())
}

async function chooseSuggestion(hit: BookSearchHit): Promise<void> {
  query.value = hit.book.title
  await submitSearch()
}

async function clearSearch(): Promise<void> {
  query.value = ''
  minPrice.value = undefined
  maxPrice.value = undefined
  stock.value = 'all'
  sort.value = 'relevance'
  suggestions.value = []
  suggestionsOpen.value = false
  await booksStore.fetchInitial(true)
}
</script>

<template>
  <section class="page-hero page-hero--catalog">
    <div class="shell page-hero__inner">
      <p class="eyebrow">Tủ sách Mộc Thư</p>
      <h1>Tìm cuốn sách<br />dành cho lúc này.</h1>
      <p>Tìm theo tên, tác giả hoặc ISBN — kể cả khi bạn gõ thiếu một vài ký tự.</p>

      <form class="book-search" role="search" @submit.prevent="submitSearch">
        <div class="book-search__query">
          <AppIcon name="search" :size="22" />
          <input
            v-model="query"
            type="search"
            maxlength="200"
            autocomplete="off"
            placeholder="Ví dụ: Clean Archtecture…"
            aria-label="Tìm kiếm sách"
            aria-autocomplete="list"
            :aria-expanded="suggestionsOpen"
            @focus="suggestionsOpen = suggestions.length > 0"
            @keydown.esc="suggestionsOpen = false"
          />
          <span v-if="suggesting" class="book-search__spinner" aria-label="Đang gợi ý" />
          <button type="submit">Tìm sách</button>

          <div v-if="suggestionsOpen" class="suggestions" role="listbox">
            <button
              v-for="hit in suggestions"
              :key="hit.book.id"
              type="button"
              role="option"
              @mousedown.prevent="chooseSuggestion(hit)"
            >
              <span
                ><strong>{{ hit.book.title }}</strong
                ><small>{{ hit.book.author }}</small></span
              >
              <small>{{ hit.book.isbn }}</small>
            </button>
            <p v-if="!suggestions.length && !suggesting">Không có gợi ý phù hợp.</p>
          </div>
        </div>

        <div class="book-search__filters">
          <label
            ><span>Giá từ</span
            ><input v-model.number="minPrice" type="number" min="0" placeholder="0"
          /></label>
          <label
            ><span>Giá đến</span
            ><input v-model.number="maxPrice" type="number" min="0" placeholder="Không giới hạn"
          /></label>
          <label>
            <span>Tồn kho</span>
            <select v-model="stock">
              <option value="all">Tất cả</option>
              <option value="in">Còn hàng</option>
              <option value="out">Hết hàng</option>
            </select>
          </label>
          <label>
            <span>Sắp xếp</span>
            <select v-model="sort">
              <option value="relevance">Liên quan nhất</option>
              <option value="newest">Mới nhất</option>
              <option value="price_asc">Giá tăng dần</option>
              <option value="price_desc">Giá giảm dần</option>
            </select>
          </label>
          <button class="book-search__apply" type="submit">Áp dụng</button>
          <button class="book-search__clear" type="button" @click="clearSearch">Xóa lọc</button>
        </div>
      </form>
    </div>
  </section>

  <section class="section catalog">
    <div class="shell">
      <div class="catalog__bar">
        <div>
          <span>{{ resultLabel }}</span>
          <small>{{ booksStore.activeSearch ? ' Elasticsearch ranking' : ' Sách mới nhất' }}</small>
        </div>
        <span class="catalog__live"><i /> Search index đồng bộ từ Book Service</span>
      </div>

      <div v-if="booksStore.loading" class="catalog-grid" aria-label="Đang tải sách">
        <div v-for="item in 8" :key="item" class="catalog-skeleton" />
      </div>
      <div v-else-if="booksStore.isEmpty && booksStore.error" class="state-card">
        <AppIcon name="book" :size="34" />
        <h2>Chưa thể tìm trong tủ sách</h2>
        <p>{{ booksStore.error }}</p>
        <button class="button button--primary" type="button" @click="submitSearch">Thử lại</button>
      </div>
      <div v-else-if="booksStore.isEmpty" class="state-card">
        <AppIcon name="search" :size="34" />
        <h2>Không tìm thấy sách</h2>
        <p>Thử bớt bộ lọc hoặc dùng một cách viết khác.</p>
        <button class="button button--outline" type="button" @click="clearSearch">
          Xóa tìm kiếm
        </button>
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
  padding-top: 70px;
  padding-bottom: 52px;
}
.page-hero__inner::after {
  position: absolute;
  z-index: 0;
  right: 5%;
  bottom: 0;
  width: 180px;
  height: 86%;
  border-radius: 100% 0 0;
  background: #c7d4bd;
  content: '';
  opacity: 0.75;
}
.page-hero h1,
.page-hero p,
.book-search {
  position: relative;
  z-index: 1;
}
.page-hero h1 {
  max-width: 700px;
  margin: 14px 0 18px;
  font-family: var(--font-display);
  font-size: clamp(3rem, 6vw, 5.4rem);
  font-weight: 550;
  letter-spacing: -0.05em;
  line-height: 0.94;
}
.page-hero > div > p:not(.eyebrow) {
  max-width: 560px;
  margin: 0;
  color: var(--color-muted);
  line-height: 1.7;
}
.book-search {
  max-width: 980px;
  margin-top: 34px;
  padding: 12px;
  border: 1px solid rgb(27 61 52 / 13%);
  border-radius: 20px;
  background: rgb(249 247 241 / 94%);
  box-shadow: 0 20px 50px rgb(27 61 52 / 10%);
}
.book-search__query {
  position: relative;
  display: grid;
  grid-template-columns: auto 1fr auto auto;
  gap: 10px;
  align-items: center;
  padding: 6px 6px 6px 12px;
  border: 1px solid var(--color-line);
  border-radius: 13px;
  background: white;
}
.book-search__query > svg {
  color: var(--color-brand);
}
.book-search__query input {
  min-width: 0;
  padding: 11px 0;
  border: 0;
  outline: 0;
  background: transparent;
  font: inherit;
}
.book-search__query > button,
.book-search__apply {
  padding: 12px 20px;
  border: 0;
  border-radius: 10px;
  color: white;
  background: var(--color-brand);
  font-weight: 750;
  cursor: pointer;
}
.book-search__spinner {
  width: 16px;
  height: 16px;
  border: 2px solid var(--color-line);
  border-top-color: var(--color-brand);
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
}
.suggestions {
  position: absolute;
  z-index: 20;
  top: calc(100% + 8px);
  right: 0;
  left: 0;
  overflow: hidden;
  border: 1px solid var(--color-line);
  border-radius: 13px;
  background: white;
  box-shadow: 0 18px 36px rgb(20 48 40 / 16%);
}
.suggestions button {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 12px 16px;
  border: 0;
  border-bottom: 1px solid var(--color-line);
  color: var(--color-ink);
  background: white;
  text-align: left;
  cursor: pointer;
}
.suggestions button:hover {
  background: #f1f4eb;
}
.suggestions strong,
.suggestions small {
  display: block;
}
.suggestions small {
  margin-top: 3px;
  color: var(--color-muted);
  font-size: 0.72rem;
}
.suggestions > p {
  margin: 0;
  padding: 18px;
  color: var(--color-muted);
}
.book-search__filters {
  display: grid;
  grid-template-columns: repeat(4, minmax(110px, 1fr)) auto auto;
  gap: 10px;
  align-items: end;
  padding: 12px 4px 2px;
}
.book-search__filters label span {
  display: block;
  margin: 0 0 5px 2px;
  color: var(--color-muted);
  font-size: 0.68rem;
  font-weight: 700;
  text-transform: uppercase;
}
.book-search__filters input,
.book-search__filters select {
  width: 100%;
  min-height: 40px;
  padding: 8px 10px;
  border: 1px solid var(--color-line);
  border-radius: 9px;
  outline: none;
  background: white;
}
.book-search__filters input:focus,
.book-search__filters select:focus {
  border-color: var(--color-brand);
}
.book-search__apply {
  display: none;
}
.book-search__clear {
  min-height: 40px;
  padding: 8px 12px;
  border: 0;
  color: var(--color-muted);
  background: transparent;
  cursor: pointer;
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
@keyframes spin {
  to {
    transform: rotate(360deg);
  }
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
  .book-search__filters {
    grid-template-columns: repeat(2, 1fr);
  }
  .book-search__apply {
    display: block;
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
  .book-search__query {
    grid-template-columns: auto 1fr auto;
  }
  .book-search__query > button {
    grid-column: 1 / -1;
  }
}
@media (max-width: 430px) {
  .catalog__bar small {
    display: none;
  }
  .book-search__filters {
    grid-template-columns: 1fr;
  }
}
</style>
