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

export interface BookInput {
  title: string
  author: string
  isbn: string
  price_cents: number
  stock: number
}

export interface CursorPagination {
  next_cursor?: string
  has_more: boolean
}

export interface BookListResponse {
  data: Book[]
  pagination: CursorPagination
}
