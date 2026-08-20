<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { useAdminBooksStore } from '@/features/books/model/books.store'
import type { Book, BookInput } from '@/features/books/model/types'
import BookFormDrawer from '@/features/books/ui/BookFormDrawer.vue'
import { useNotificationStore } from '@/features/notifications/model/notification.store'
import { formatDateTime, formatPrice } from '@/shared/lib/format'
import AppIcon from '@/shared/ui/AppIcon.vue'
import ConfirmDialog from '@/shared/ui/ConfirmDialog.vue'

const store = useAdminBooksStore()
const notifications = useNotificationStore()
const query = ref('')
const drawerOpen = ref(false)
const editingBook = ref<Book>()
const deletingBook = ref<Book>()

const filteredBooks = computed(() => {
  const keyword = query.value.trim().toLocaleLowerCase('vi')
  if (!keyword) return store.books
  return store.books.filter((book) =>
    [book.title, book.author, book.isbn].some((value) =>
      value.toLocaleLowerCase('vi').includes(keyword),
    ),
  )
})

onMounted(() => store.fetchInitial())

function openCreate(): void {
  editingBook.value = undefined
  drawerOpen.value = true
}

function openEdit(book: Book): void {
  editingBook.value = book
  drawerOpen.value = true
}

async function save(payload: BookInput): Promise<void> {
  try {
    if (editingBook.value) {
      await store.update(editingBook.value.id, payload)
      notifications.show('Đã cập nhật sách trên storefront.')
    } else {
      await store.create(payload)
      notifications.show('Đã thêm sách mới vào storefront.')
    }
    drawerOpen.value = false
  } catch {
    notifications.show(store.error || 'Không thể lưu sách.', 'danger')
  }
}

async function confirmDelete(): Promise<void> {
  if (!deletingBook.value) return
  try {
    await store.remove(deletingBook.value.id)
    notifications.show(`Đã xóa “${deletingBook.value.title}”.`)
    deletingBook.value = undefined
  } catch {
    notifications.show(store.error || 'Không thể xóa sách.', 'danger')
  }
}
</script>

<template>
  <div class="page-stack">
    <section class="page-heading">
      <div>
        <p class="eyebrow">Catalog operations</p>
        <h2>Danh mục sách</h2>
        <p>Quản lý nội dung, giá bán và số lượng hiển thị trên storefront.</p>
      </div>
      <button class="button button--primary" type="button" @click="openCreate">
        <AppIcon name="plus" :size="17" /> Thêm sách
      </button>
    </section>

    <section class="catalog-panel">
      <header class="catalog-toolbar">
        <label class="search-box"
          ><AppIcon name="search" :size="18" /><span class="sr-only">Tìm kiếm sách</span
          ><input v-model="query" type="search" placeholder="Tìm theo tên, tác giả hoặc ISBN…"
        /></label>
        <div class="catalog-toolbar__meta">
          <span>{{ filteredBooks.length }} kết quả</span
          ><button
            class="icon-button"
            type="button"
            title="Làm mới"
            :disabled="store.loading"
            @click="store.fetchInitial(true)"
          >
            <AppIcon name="refresh" :size="18" />
          </button>
        </div>
      </header>

      <p v-if="store.error" class="inline-error catalog-error">{{ store.error }}</p>

      <div class="table-wrap">
        <table class="books-table">
          <thead>
            <tr>
              <th>Sách</th>
              <th>ISBN</th>
              <th>Giá bán</th>
              <th>Tồn kho</th>
              <th>Cập nhật</th>
              <th><span class="sr-only">Thao tác</span></th>
            </tr>
          </thead>
          <tbody v-if="store.loading">
            <tr v-for="item in 6" :key="item" class="table-skeleton">
              <td colspan="6"><i /></td>
            </tr>
          </tbody>
          <tbody v-else-if="filteredBooks.length">
            <tr v-for="book in filteredBooks" :key="book.id">
              <td>
                <div class="table-book">
                  <span class="book-monogram">{{ book.title.slice(0, 1).toUpperCase() }}</span>
                  <p>
                    <strong>{{ book.title }}</strong
                    ><small>{{ book.author }}</small>
                  </p>
                </div>
              </td>
              <td>
                <code>{{ book.isbn }}</code>
              </td>
              <td>
                <strong>{{ formatPrice(book.price_cents) }}</strong>
              </td>
              <td>
                <span
                  class="stock-badge"
                  :class="{
                    'stock-badge--low': book.stock <= 5,
                    'stock-badge--empty': book.stock === 0,
                  }"
                  >{{ book.stock === 0 ? 'Hết hàng' : `${book.stock} cuốn` }}</span
                >
              </td>
              <td>
                <span class="date-cell">{{ formatDateTime(book.updated_at) }}</span>
              </td>
              <td>
                <div class="row-actions">
                  <button type="button" title="Sửa sách" @click="openEdit(book)">
                    <AppIcon name="edit" :size="17" /></button
                  ><button
                    class="row-actions__delete"
                    type="button"
                    title="Xóa sách"
                    @click="deletingBook = book"
                  >
                    <AppIcon name="trash" :size="17" />
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>

        <div v-if="!store.loading && !filteredBooks.length" class="empty-state">
          <span><AppIcon name="search" :size="26" /></span>
          <h3>{{ query ? 'Không tìm thấy sách' : 'Danh mục đang trống' }}</h3>
          <p>
            {{ query ? 'Thử một từ khóa khác.' : 'Thêm cuốn sách đầu tiên để bắt đầu bán hàng.' }}
          </p>
          <button v-if="!query" class="button button--primary" type="button" @click="openCreate">
            <AppIcon name="plus" :size="17" /> Thêm sách
          </button>
        </div>
      </div>

      <footer v-if="store.hasMore && !query" class="catalog-footer">
        <button
          class="button button--secondary"
          type="button"
          :disabled="store.loadingMore"
          @click="store.fetchNext"
        >
          <span v-if="store.loadingMore" class="spinner" />{{
            store.loadingMore ? 'Đang tải…' : 'Tải thêm sách'
          }}
        </button>
      </footer>
    </section>

    <BookFormDrawer
      :open="drawerOpen"
      :book="editingBook"
      :saving="store.saving"
      @close="drawerOpen = false"
      @save="save"
    />
    <ConfirmDialog
      :open="Boolean(deletingBook)"
      title="Xóa sách khỏi cửa hàng?"
      :message="`“${deletingBook?.title || ''}” sẽ biến mất khỏi storefront. Hành động này không thể hoàn tác.`"
      :busy="Boolean(store.deletingId)"
      @cancel="deletingBook = undefined"
      @confirm="confirmDelete"
    />
  </div>
</template>
