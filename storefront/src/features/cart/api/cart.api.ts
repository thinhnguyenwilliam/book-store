import { apiRequest } from '@/shared/api/http-client'

export interface ServerCartItem {
  id: string
  book_id: string
  quantity: number
  created_at: string
  updated_at: string
}

interface CartListResponse {
  data: ServerCartItem[]
}

export async function listCart(signal?: AbortSignal): Promise<ServerCartItem[]> {
  const response = await apiRequest<CartListResponse>('/api/v1/cart/items', {
    ...(signal ? { signal } : {}),
  })
  return response.data
}

export function addCartItem(bookID: string, quantity: number): Promise<ServerCartItem> {
  return apiRequest<ServerCartItem>('/api/v1/cart/items', {
    method: 'POST',
    data: { bookId: bookID, quantity },
  })
}

export function updateCartItem(itemID: string, quantity: number): Promise<ServerCartItem> {
  return apiRequest<ServerCartItem>(`/api/v1/cart/items/${encodeURIComponent(itemID)}`, {
    method: 'PUT',
    data: { quantity },
  })
}

export function removeCartItem(itemID: string): Promise<void> {
  return apiRequest<void>(`/api/v1/cart/items/${encodeURIComponent(itemID)}`, {
    method: 'DELETE',
  })
}
