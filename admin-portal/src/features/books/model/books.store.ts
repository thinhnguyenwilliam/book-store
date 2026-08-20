import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { ApiError } from '@/shared/api/http-client'
import * as booksApi from '../api/books.api'
import type { Book, BookInput } from './types'

const PAGE_SIZE = 30

function errorMessage(error: unknown, fallback: string): string {
  if (!(error instanceof ApiError)) return fallback
  return error.traceId ? `${error.message} · Trace: ${error.traceId}` : error.message
}

export const useAdminBooksStore = defineStore('admin-books', () => {
  const books = ref<Book[]>([])
  const nextCursor = ref<string>()
  const hasMore = ref(false)
  const loading = ref(false)
  const loadingMore = ref(false)
  const saving = ref(false)
  const deletingId = ref<string>()
  const error = ref('')
  let activeRequest: AbortController | undefined

  const totalLoaded = computed(() => books.value.length)
  const inventoryUnits = computed(() => books.value.reduce((sum, book) => sum + book.stock, 0))
  const lowStockCount = computed(() => books.value.filter((book) => book.stock <= 5).length)
  const inventoryValueCents = computed(() =>
    books.value.reduce((sum, book) => sum + book.price_cents * book.stock, 0),
  )

  async function fetchInitial(force = false): Promise<void> {
    if (loading.value || (!force && books.value.length > 0)) return
    activeRequest?.abort()
    activeRequest = new AbortController()
    loading.value = true
    error.value = ''
    try {
      const response = await booksApi.listBooks({ limit: PAGE_SIZE, signal: activeRequest.signal })
      books.value = response.data
      nextCursor.value = response.pagination.next_cursor
      hasMore.value = response.pagination.has_more
    } catch (requestError) {
      if (activeRequest.signal.aborted) return
      error.value = errorMessage(requestError, 'Không thể tải danh mục sách.')
    } finally {
      loading.value = false
    }
  }

  async function fetchNext(): Promise<void> {
    if (loadingMore.value || !hasMore.value || !nextCursor.value) return
    loadingMore.value = true
    error.value = ''
    try {
      const response = await booksApi.listBooks({ limit: PAGE_SIZE, cursor: nextCursor.value })
      const knownIDs = new Set(books.value.map((book) => book.id))
      books.value.push(...response.data.filter((book) => !knownIDs.has(book.id)))
      nextCursor.value = response.pagination.next_cursor
      hasMore.value = response.pagination.has_more
    } catch (requestError) {
      error.value = errorMessage(requestError, 'Không thể tải thêm sách.')
    } finally {
      loadingMore.value = false
    }
  }

  async function create(payload: BookInput): Promise<Book> {
    saving.value = true
    error.value = ''
    try {
      const book = await booksApi.createBook(payload)
      books.value.unshift(book)
      return book
    } catch (requestError) {
      error.value = errorMessage(requestError, 'Không thể tạo sách.')
      throw requestError
    } finally {
      saving.value = false
    }
  }

  async function update(id: string, payload: BookInput): Promise<Book> {
    saving.value = true
    error.value = ''
    try {
      const updated = await booksApi.updateBook(id, payload)
      const index = books.value.findIndex((book) => book.id === id)
      if (index >= 0) books.value[index] = updated
      return updated
    } catch (requestError) {
      error.value = errorMessage(requestError, 'Không thể cập nhật sách.')
      throw requestError
    } finally {
      saving.value = false
    }
  }

  async function remove(id: string): Promise<void> {
    deletingId.value = id
    error.value = ''
    try {
      await booksApi.deleteBook(id)
      books.value = books.value.filter((book) => book.id !== id)
    } catch (requestError) {
      error.value = errorMessage(requestError, 'Không thể xóa sách.')
      throw requestError
    } finally {
      deletingId.value = undefined
    }
  }

  return {
    books,
    hasMore,
    loading,
    loadingMore,
    saving,
    deletingId,
    error,
    totalLoaded,
    inventoryUnits,
    lowStockCount,
    inventoryValueCents,
    fetchInitial,
    fetchNext,
    create,
    update,
    remove,
  }
})
