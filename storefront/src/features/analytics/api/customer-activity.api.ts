import { apiRequest } from '@/shared/api/http-client'

export type CustomerActivityType =
  | 'book.viewed'
  | 'book.searched'
  | 'book.added_to_cart'
  | 'book.removed_from_cart'
  | 'checkout.started'

export interface CustomerActivityPayload {
  event_type: CustomerActivityType
  anonymous_id: string
  session_id: string
  book_id?: string
  query?: string
  quantity?: number
}

export function sendCustomerActivity(payload: CustomerActivityPayload): Promise<void> {
  return apiRequest<void>('/api/v1/customer-activity', {
    method: 'POST',
    data: payload,
    // Analytics must not trigger an auth refresh or interrupt the user journey.
    skipAuthRefresh: true,
  })
}
