import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import type { Book } from '@/features/books/model/types'
import { useCartStore } from './cart.store'

const book: Book = {
  id: 'book-1',
  title: 'Clean Architecture',
  author: 'Robert C. Martin',
  isbn: '9780134494166',
  price_cents: 3999,
  stock: 2,
  created_at: '2026-08-20T00:00:00Z',
  updated_at: '2026-08-20T00:00:00Z',
}

describe('cart store', () => {
  beforeEach(() => {
    localStorage.clear()
    setActivePinia(createPinia())
  })

  it('adds items and never exceeds available stock', () => {
    const cart = useCartStore()
    cart.add(book)
    cart.add(book)
    cart.add(book)

    expect(cart.itemCount).toBe(2)
    expect(cart.subtotalCents).toBe(7998)
  })
})
