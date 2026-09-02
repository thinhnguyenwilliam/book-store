import { apiRequest } from '@/shared/api/http-client'

export interface DailyOrderMetric {
  date: string
  created: number
  confirmed: number
  cancelled: number
}

export interface OrderAnalytics {
  from: string
  to: string
  total_orders: number
  confirmed_orders: number
  cancelled_orders: number
  payment_attempts: number
  payment_succeeded: number
  payment_failed: number
  stock_reservation_failed: number
  payment_success_rate: number
  average_confirmation_seconds: number
  daily: DailyOrderMetric[]
  last_event_at?: string
}

export interface BookActivityMetric {
  book_id: string
  views: number
  cart_adds: number
  comments: number
  score: number
}

export interface CustomerActivityAnalytics {
  from: string
  to: string
  total_events: number
  unique_actors: number
  abandoned_carts: number
  view_to_cart_rate: number
  cart_to_checkout_rate: number
  checkout_to_order_rate: number
  event_counts: Array<{ event_type: string; count: number }>
  top_books: BookActivityMetric[]
  last_event_at?: string
}

export function getOrderAnalytics(signal?: AbortSignal): Promise<OrderAnalytics> {
  return apiRequest<OrderAnalytics>('/api/v1/admin/analytics/orders', { signal })
}

export function getCustomerActivityAnalytics(
  signal?: AbortSignal,
): Promise<CustomerActivityAnalytics> {
  return apiRequest<CustomerActivityAnalytics>('/api/v1/admin/analytics/customer-activity', {
    params: { limit: 10 },
    signal,
  })
}
