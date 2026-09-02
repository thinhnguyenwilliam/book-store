import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'

import { getBook } from '@/features/books/api/books.api'
import type { Book } from '@/features/books/model/types'
import * as cartApi from '../api/cart.api'

const CART_KEY = 'bookstore.cart'

export interface CartItem {
  id?: string
  book: Book
  quantity: number
  updatedAt?: string
}

function loadGuestCart(): CartItem[] {
  try {
    return JSON.parse(localStorage.getItem(CART_KEY) || '[]') as CartItem[]
  } catch {
    return []
  }
}

export const useCartStore = defineStore('cart', () => {
  const items = ref<CartItem[]>(loadGuestCart())
  const mode = ref<'guest' | 'server'>('guest')
  const loading = ref(false)
  let syncPromise: Promise<void> | undefined

  const itemCount = computed(() => items.value.reduce((total, item) => total + item.quantity, 0))
  const subtotalCents = computed(() =>
    items.value.reduce((total, item) => total + item.book.price_cents * item.quantity, 0),
  )
  const isEmpty = computed(() => items.value.length === 0)

  watch(
    items,
    (value) => {
      if (mode.value === 'guest') localStorage.setItem(CART_KEY, JSON.stringify(value))
    },
    { deep: true },
  )

  async function hydrate(serverItems: cartApi.ServerCartItem[]): Promise<CartItem[]> {
    const hydrated = await Promise.all(
      serverItems.map(async (item): Promise<CartItem | undefined> => {
        try {
          return {
            id: item.id,
            book: await getBook(item.book_id),
            quantity: item.quantity,
            updatedAt: item.updated_at,
          }
        } catch {
          return undefined
        }
      }),
    )
    return hydrated.filter((item): item is CartItem => Boolean(item))
  }

  async function syncAuthenticated(force = false): Promise<void> {
    if (mode.value === 'server' && !force) return
    if (syncPromise) return syncPromise
    syncPromise = (async () => {
      loading.value = true
      try {
        const guestItems = mode.value === 'guest' ? loadGuestCart() : []
        let serverItems = await cartApi.listCart()
        for (const guest of guestItems) {
          const existing = serverItems.find((item) => item.book_id === guest.book.id)
          const desired = Math.min(Math.max(existing?.quantity ?? 0, guest.quantity), 100)
          if (existing && desired !== existing.quantity) {
            await cartApi.updateCartItem(existing.id, desired)
          } else if (!existing && desired > 0) {
            await cartApi.addCartItem(guest.book.id, desired)
          }
        }
        if (guestItems.length) serverItems = await cartApi.listCart()
        items.value = await hydrate(serverItems)
        mode.value = 'server'
        localStorage.removeItem(CART_KEY)
      } finally {
        loading.value = false
        syncPromise = undefined
      }
    })()
    return syncPromise
  }

  function switchToGuest(): void {
    mode.value = 'guest'
    items.value = loadGuestCart()
  }

  async function add(book: Book, authenticated = false): Promise<void> {
    if (book.stock < 1) return
    if (!authenticated) {
      const existing = items.value.find((item) => item.book.id === book.id)
      if (existing) {
        existing.quantity = Math.min(existing.quantity + 1, book.stock)
        existing.book = book
      } else {
        items.value.push({ book, quantity: 1 })
      }
      return
    }
    await syncAuthenticated()
    const result = await cartApi.addCartItem(book.id, 1)
    const existing = items.value.find((item) => item.book.id === book.id)
    if (existing) {
      existing.id = result.id
      existing.quantity = result.quantity
      existing.updatedAt = result.updated_at
      existing.book = book
    } else {
      items.value.push({
        id: result.id,
        book,
        quantity: result.quantity,
        updatedAt: result.updated_at,
      })
    }
  }

  async function setQuantity(
    bookID: string,
    quantity: number,
    authenticated = false,
  ): Promise<void> {
    const item = items.value.find((candidate) => candidate.book.id === bookID)
    if (!item) return
    if (quantity <= 0) return remove(bookID, authenticated)
    const desired = Math.min(Math.floor(quantity), item.book.stock, 100)
    if (!authenticated) {
      item.quantity = desired
      return
    }
    await syncAuthenticated()
    if (!item.id) return
    const result = await cartApi.updateCartItem(item.id, desired)
    item.quantity = result.quantity
    item.updatedAt = result.updated_at
  }

  async function remove(bookID: string, authenticated = false): Promise<void> {
    const item = items.value.find((candidate) => candidate.book.id === bookID)
    if (!item) return
    if (authenticated) {
      await syncAuthenticated()
      if (item.id) await cartApi.removeCartItem(item.id)
    }
    items.value = items.value.filter((candidate) => candidate.book.id !== bookID)
  }

  function clear(): void {
    items.value = []
    if (mode.value === 'guest') localStorage.removeItem(CART_KEY)
  }

  return {
    items,
    mode,
    loading,
    itemCount,
    subtotalCents,
    isEmpty,
    syncAuthenticated,
    switchToGuest,
    add,
    setQuantity,
    remove,
    clear,
  }
})
