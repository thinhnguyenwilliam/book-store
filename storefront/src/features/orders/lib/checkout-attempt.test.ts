import { beforeEach, describe, expect, it } from 'vitest'

import { clearCheckoutAttempt, getCheckoutAttempt, saveCheckoutOrder } from './checkout-attempt'

describe('checkout attempt', () => {
  beforeEach(() => sessionStorage.clear())

  it('reuses keys across retries and stores the created order', () => {
    const first = getCheckoutAttempt()
    const retry = getCheckoutAttempt()
    const withOrder = saveCheckoutOrder(retry, 'order-1')

    expect(retry).toEqual(first)
    expect(withOrder.orderID).toBe('order-1')
    expect(getCheckoutAttempt()).toEqual(withOrder)
  })

  it('creates fresh keys after a completed or rejected attempt', () => {
    const first = getCheckoutAttempt()
    clearCheckoutAttempt()
    const next = getCheckoutAttempt()

    expect(next.orderKey).not.toBe(first.orderKey)
    expect(next.paymentKey).not.toBe(first.paymentKey)
  })
})
