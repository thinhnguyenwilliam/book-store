import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as booksApi from '../api/books.api'
import type { Book } from './types'
import { useAdminBooksStore } from './books.store'

vi.mock('../api/books.api', () => ({
  listBooks: vi.fn(),
  createBook: vi.fn(),
  updateBook: vi.fn(),
  deleteBook: vi.fn(),
}))

const firstBook: Book = {
  id: 'book-1',
  title: 'Clean Architecture',
  author: 'Robert C. Martin',
  isbn: '9780134494166',
  price_cents: 3999,
  stock: 4,
  created_at: '2026-08-20T09:00:00Z',
  updated_at: '2026-08-20T09:00:00Z',
}

describe('admin books store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('loads cursor data and computes inventory indicators', async () => {
    vi.mocked(booksApi.listBooks).mockResolvedValue({
      data: [firstBook],
      pagination: { has_more: false },
    })
    const store = useAdminBooksStore()

    await store.fetchInitial()

    expect(store.books).toEqual([firstBook])
    expect(store.inventoryUnits).toBe(4)
    expect(store.lowStockCount).toBe(1)
    expect(store.inventoryValueCents).toBe(15_996)
  })

  it('updates local catalog only after API mutation succeeds', async () => {
    vi.mocked(booksApi.listBooks).mockResolvedValue({
      data: [firstBook],
      pagination: { has_more: false },
    })
    vi.mocked(booksApi.updateBook).mockResolvedValue({ ...firstBook, stock: 12 })
    const store = useAdminBooksStore()
    await store.fetchInitial()

    await store.update(firstBook.id, {
      title: firstBook.title,
      author: firstBook.author,
      isbn: firstBook.isbn,
      price_cents: firstBook.price_cents,
      stock: 12,
    })

    expect(store.books[0]?.stock).toBe(12)
  })
})
