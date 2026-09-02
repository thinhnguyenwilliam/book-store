export interface Book {
  id: string
  title: string
  author: string
  isbn: string
  price_cents: number
  stock: number
  created_at: string
  updated_at: string
}

export interface CursorPagination {
  next_cursor?: string
  has_more: boolean
}

export interface BookListResponse {
  data: Book[]
  pagination: CursorPagination
}

export interface BookSearchHit {
  book: Book
  score: number
  highlights?: Record<string, string>
}

export interface BookSearchResponse {
  data: BookSearchHit[]
  pagination: CursorPagination
  total: number
  took_ms: number
}

export interface BookSuggestionResponse {
  data: BookSearchHit[]
  took_ms: number
}
