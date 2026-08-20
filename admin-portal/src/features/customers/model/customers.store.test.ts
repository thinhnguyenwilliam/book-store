import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as customersApi from '../api/customers.api'
import type { Customer } from './types'
import { useCustomersStore } from './customers.store'

vi.mock('../api/customers.api', () => ({
  listCustomers: vi.fn(),
  getCustomer: vi.fn(),
  updateCustomer: vi.fn(),
  deleteCustomer: vi.fn(),
}))

const customer: Customer = {
  id: 'customer-1',
  email: 'reader@example.com',
  display_name: 'Reader',
  created_at: '2026-08-20T09:00:00Z',
  updated_at: '2026-08-20T09:00:00Z',
}

describe('customers store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('loads a cursor page', async () => {
    vi.mocked(customersApi.listCustomers).mockResolvedValue({
      data: [customer],
      pagination: { has_more: true, next_cursor: 'next-page' },
    })
    const store = useCustomersStore()

    await store.fetchInitial()

    expect(store.customers).toEqual([customer])
    expect(store.hasMore).toBe(true)
  })

  it('updates the matching profile after the API succeeds', async () => {
    vi.mocked(customersApi.listCustomers).mockResolvedValue({
      data: [customer],
      pagination: { has_more: false },
    })
    vi.mocked(customersApi.updateCustomer).mockResolvedValue({
      ...customer,
      display_name: 'Reader Updated',
    })
    const store = useCustomersStore()
    await store.fetchInitial()

    await store.update(customer.id, { display_name: 'Reader Updated' })

    expect(store.customers[0]?.display_name).toBe('Reader Updated')
  })

  it('removes the profile locally after deletion is accepted', async () => {
    vi.mocked(customersApi.listCustomers).mockResolvedValue({
      data: [customer],
      pagination: { has_more: false },
    })
    vi.mocked(customersApi.deleteCustomer).mockResolvedValue({ status: 'accepted' })
    const store = useCustomersStore()
    await store.fetchInitial()

    await store.remove(customer.id)

    expect(store.customers).toEqual([])
  })
})
