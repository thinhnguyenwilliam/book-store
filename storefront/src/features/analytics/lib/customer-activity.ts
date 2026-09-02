import {
  sendCustomerActivity,
  type CustomerActivityPayload,
  type CustomerActivityType,
} from '../api/customer-activity.api'

const ANONYMOUS_ID_KEY = 'bookstore.analytics.anonymous-id'
const SESSION_ID_KEY = 'bookstore.analytics.session-id'

export function trackBookViewed(bookID: string): void {
  track({ event_type: 'book.viewed', book_id: bookID })
}

export function trackBookSearched(query: string): void {
  const normalized = query.trim().slice(0, 200)
  if (normalized.length >= 2) track({ event_type: 'book.searched', query: normalized })
}

export function trackBookAddedToCart(bookID: string, quantity = 1): void {
  track({ event_type: 'book.added_to_cart', book_id: bookID, quantity })
}

export function trackBookRemovedFromCart(bookID: string, quantity = 1): void {
  track({ event_type: 'book.removed_from_cart', book_id: bookID, quantity })
}

export function trackCheckoutStarted(): void {
  track({ event_type: 'checkout.started' })
}

function track(activity: {
  event_type: CustomerActivityType
  book_id?: string
  query?: string
  quantity?: number
}): void {
  if (typeof window === 'undefined') return
  const payload: CustomerActivityPayload = {
    ...activity,
    anonymous_id: stableID(localStorage, ANONYMOUS_ID_KEY),
    session_id: stableID(sessionStorage, SESSION_ID_KEY),
  }
  // Behavior analytics is intentionally best effort and must never surface an
  // error toast or block navigation, cart actions, comments, or checkout.
  void sendCustomerActivity(payload).catch(() => undefined)
}

function stableID(storage: Storage, key: string): string {
  const current = storage.getItem(key)
  if (current) return current
  const value = crypto.randomUUID()
  storage.setItem(key, value)
  return value
}
