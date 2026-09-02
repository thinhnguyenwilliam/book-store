import type { Payment } from '@/features/payments/model/types'
import { apiRequest } from '@/shared/api/http-client'

export interface CheckoutOrderItem {
  id: string
  book_id: string
  seller_id: string
  title: string
  unit_price_cents: number
  quantity: number
  subtotal_cents: number
}

export interface CheckoutOrder {
  id: string
  status:
    | 'pending'
    | 'stock_reserved'
    | 'payment_pending'
    | 'confirmed'
    | 'cancelled'
    | 'compensation_pending'
  total_cents: number
  currency: string
  items: CheckoutOrderItem[]
  payment_id?: string
  failure_reason?: string
  reservation_expires_at: string
  created_at: string
  updated_at: string
}

export function createOrder(idempotencyKey: string): Promise<CheckoutOrder> {
  return apiRequest<CheckoutOrder>('/api/v1/orders', {
    method: 'POST',
    headers: { 'Idempotency-Key': idempotencyKey },
  })
}

export function payOrder(
  orderID: string,
  provider: 'wallet' | 'vnpay',
  idempotencyKey: string,
): Promise<Payment> {
  return apiRequest<Payment>('/api/v1/payments', {
    method: 'POST',
    headers: { 'Idempotency-Key': idempotencyKey },
    data: { orderId: orderID, provider, locale: 'vn' },
  })
}

export function cancelOrder(orderID: string): Promise<CheckoutOrder> {
  return apiRequest<CheckoutOrder>(`/api/v1/orders/${encodeURIComponent(orderID)}/cancel`, {
    method: 'PUT',
  })
}
