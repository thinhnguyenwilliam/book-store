const CHECKOUT_ATTEMPT_KEY = 'bookstore.checkout-attempt'

export interface CheckoutAttempt {
  orderKey: string
  paymentKey: string
  orderID?: string
}

export function getCheckoutAttempt(): CheckoutAttempt {
  try {
    const existing = JSON.parse(
      sessionStorage.getItem(CHECKOUT_ATTEMPT_KEY) || 'null',
    ) as CheckoutAttempt | null
    if (existing?.orderKey && existing.paymentKey) return existing
  } catch {
    // Replace malformed client state with a safe fresh attempt.
  }
  const attempt = {
    orderKey: `order:${crypto.randomUUID()}`,
    paymentKey: `payment:${crypto.randomUUID()}`,
  }
  sessionStorage.setItem(CHECKOUT_ATTEMPT_KEY, JSON.stringify(attempt))
  return attempt
}

export function saveCheckoutOrder(attempt: CheckoutAttempt, orderID: string): CheckoutAttempt {
  const updated = { ...attempt, orderID }
  sessionStorage.setItem(CHECKOUT_ATTEMPT_KEY, JSON.stringify(updated))
  return updated
}

export function clearCheckoutAttempt(): void {
  sessionStorage.removeItem(CHECKOUT_ATTEMPT_KEY)
}
