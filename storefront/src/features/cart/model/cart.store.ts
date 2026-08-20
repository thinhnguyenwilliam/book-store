import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'

import type { Book } from '@/features/books/model/types'

const CART_KEY = 'bookstore.cart'

export interface CartItem {
  book: Book
  quantity: number
}

function loadCart(): CartItem[] {
  try {
    return JSON.parse(localStorage.getItem(CART_KEY) || '[]') as CartItem[]
  } catch {
    return []
  }
}

export const useCartStore = defineStore('cart', () => {
  const items = ref<CartItem[]>(loadCart())

  const itemCount = computed(() => items.value.reduce((total, item) => total + item.quantity, 0))
  const subtotalCents = computed(() =>
    items.value.reduce((total, item) => total + item.book.price_cents * item.quantity, 0),
  )
  const isEmpty = computed(() => items.value.length === 0)

  function add(book: Book): void {
    if (book.stock < 1) return
    const existing = items.value.find((item) => item.book.id === book.id)
    if (existing) {
      existing.quantity = Math.min(existing.quantity + 1, book.stock)
      existing.book = book
      return
    }
    items.value.push({ book, quantity: 1 })
  }

  function setQuantity(bookID: string, quantity: number): void {
    const item = items.value.find((candidate) => candidate.book.id === bookID)
    if (!item) return
    if (quantity <= 0) {
      remove(bookID)
      return
    }
    item.quantity = Math.min(Math.floor(quantity), item.book.stock)
  }

  function remove(bookID: string): void {
    items.value = items.value.filter((item) => item.book.id !== bookID)
  }

  function clear(): void {
    items.value = []
  }

  watch(items, (value) => localStorage.setItem(CART_KEY, JSON.stringify(value)), { deep: true })

  return { items, itemCount, subtotalCents, isEmpty, add, setQuantity, remove, clear }
})
