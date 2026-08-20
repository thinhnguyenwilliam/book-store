import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { ApiError } from '@/shared/api/http-client'
import { listBooks } from '../api/books.api'
import type { Book } from './types'

const PAGE_SIZE = 12

export const useBooksStore = defineStore('books', () => {
  const books = ref<Book[]>([])
  const nextCursor = ref<string>()
  const hasMore = ref(false)
  const loading = ref(false)
  const loadingMore = ref(false)
  const error = ref('')
  let activeRequest: AbortController | undefined

  const isEmpty = computed(() => !loading.value && books.value.length === 0)

  async function fetchInitial(force = false): Promise<void> {
    if (loading.value || (!force && books.value.length > 0)) return

    activeRequest?.abort()
    activeRequest = new AbortController()
    loading.value = true
    error.value = ''
    try {
      const response = await listBooks({ limit: PAGE_SIZE, signal: activeRequest.signal })
      books.value = response.data
      nextCursor.value = response.pagination.next_cursor
      hasMore.value = response.pagination.has_more
    } catch (requestError) {
      if (activeRequest.signal.aborted) return
      error.value =
        requestError instanceof ApiError ? requestError.message : 'Không thể tải danh sách sách.'
    } finally {
      loading.value = false
    }
  }

  async function fetchNext(): Promise<void> {
    if (loadingMore.value || !hasMore.value || !nextCursor.value) return

    loadingMore.value = true
    error.value = ''
    try {
      const response = await listBooks({ limit: PAGE_SIZE, cursor: nextCursor.value })
      const seen = new Set(books.value.map((book) => book.id))
      books.value.push(...response.data.filter((book) => !seen.has(book.id)))
      nextCursor.value = response.pagination.next_cursor
      hasMore.value = response.pagination.has_more
    } catch (requestError) {
      error.value =
        requestError instanceof ApiError ? requestError.message : 'Không thể tải thêm sách.'
    } finally {
      loadingMore.value = false
    }
  }

  return { books, hasMore, loading, loadingMore, error, isEmpty, fetchInitial, fetchNext }
})
