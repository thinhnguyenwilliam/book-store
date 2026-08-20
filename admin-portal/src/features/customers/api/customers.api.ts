import { apiRequest } from '@/shared/api/http-client'

import type { Customer, CustomerInput, CustomerListResponse } from '../model/types'

export interface ListCustomersParams {
  limit?: number
  cursor?: string
  signal?: AbortSignal
}

export function listCustomers({ limit = 30, cursor, signal }: ListCustomersParams = {}) {
  return apiRequest<CustomerListResponse>('/api/v1/admin/customers', {
    params: cursor ? { limit, cursor } : { limit },
    ...(signal ? { signal } : {}),
  })
}

export function getCustomer(id: string) {
  return apiRequest<Customer>(`/api/v1/admin/customers/${encodeURIComponent(id)}`)
}

export function updateCustomer(id: string, payload: CustomerInput) {
  return apiRequest<Customer>(`/api/v1/admin/customers/${encodeURIComponent(id)}`, {
    method: 'PUT',
    data: payload,
  })
}

export function deleteCustomer(id: string) {
  return apiRequest<{ status: 'accepted' }>(`/api/v1/admin/customers/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}
