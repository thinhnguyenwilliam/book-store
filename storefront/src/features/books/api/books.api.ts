import { apiRequest } from '@/shared/api/http-client'

import type { Book, BookListResponse } from '../model/types'

export interface ListBooksParams {
  limit?: number
  cursor?: string
  signal?: AbortSignal
}

export function listBooks({ limit = 12, cursor, signal }: ListBooksParams = {}) {
  return apiRequest<BookListResponse>('/api/v1/books', {
    params: cursor ? { limit, cursor } : { limit },
    ...(signal ? { signal } : {}),
  })
}

export function getBook(id: string, signal?: AbortSignal) {
  return apiRequest<Book>(`/api/v1/books/${encodeURIComponent(id)}`, signal ? { signal } : {})
}
