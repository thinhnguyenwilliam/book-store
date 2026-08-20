import { apiRequest } from '@/shared/api/http-client'

import type { Book, BookInput, BookListResponse } from '../model/types'

export interface ListBooksParams {
  limit?: number
  cursor?: string
  signal?: AbortSignal
}

export function listBooks({ limit = 30, cursor, signal }: ListBooksParams = {}) {
  return apiRequest<BookListResponse>('/api/v1/books', {
    params: cursor ? { limit, cursor } : { limit },
    ...(signal ? { signal } : {}),
  })
}

export function createBook(payload: BookInput) {
  return apiRequest<Book>('/api/v1/admin/books', { method: 'POST', data: payload })
}

export function updateBook(id: string, payload: BookInput) {
  return apiRequest<Book>(`/api/v1/admin/books/${encodeURIComponent(id)}`, {
    method: 'PUT',
    data: payload,
  })
}

export function deleteBook(id: string) {
  return apiRequest<void>(`/api/v1/admin/books/${encodeURIComponent(id)}`, { method: 'DELETE' })
}
