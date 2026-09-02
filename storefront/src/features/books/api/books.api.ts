import { apiRequest } from '@/shared/api/http-client'

import type {
  Book,
  BookListResponse,
  BookSearchResponse,
  BookSuggestionResponse,
} from '../model/types'

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

export type BookSearchSort = 'relevance' | 'newest' | 'price_asc' | 'price_desc'

export interface SearchBooksParams {
  query?: string
  limit?: number
  cursor?: string
  minPriceCents?: number
  maxPriceCents?: number
  inStock?: boolean
  sellerID?: string
  author?: string
  sort?: BookSearchSort
  signal?: AbortSignal
}

export function searchBooks({
  query,
  limit = 12,
  cursor,
  minPriceCents,
  maxPriceCents,
  inStock,
  sellerID,
  author,
  sort = 'relevance',
  signal,
}: SearchBooksParams) {
  return apiRequest<BookSearchResponse>('/api/v1/books/search', {
    params: {
      ...(query?.trim() ? { q: query.trim() } : {}),
      limit,
      ...(cursor ? { cursor } : {}),
      ...(minPriceCents !== undefined ? { min_price_cents: minPriceCents } : {}),
      ...(maxPriceCents !== undefined ? { max_price_cents: maxPriceCents } : {}),
      ...(inStock !== undefined ? { in_stock: inStock } : {}),
      ...(sellerID?.trim() ? { seller_id: sellerID.trim() } : {}),
      ...(author?.trim() ? { author: author.trim() } : {}),
      sort,
    },
    ...(signal ? { signal } : {}),
  })
}

export function suggestBooks(query: string, signal?: AbortSignal) {
  return apiRequest<BookSuggestionResponse>('/api/v1/books/suggest', {
    params: { q: query.trim(), limit: 8 },
    ...(signal ? { signal } : {}),
    skipAuthRefresh: true,
  })
}
