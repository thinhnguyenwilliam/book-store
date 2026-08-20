import { ref } from 'vue'
import { defineStore } from 'pinia'

import { ApiError } from '@/shared/api/http-client'
import * as customersApi from '../api/customers.api'
import type { Customer, CustomerInput } from './types'

const PAGE_SIZE = 30

function errorMessage(error: unknown, fallback: string): string {
  if (!(error instanceof ApiError)) return fallback
  return error.traceId ? `${error.message} · Trace: ${error.traceId}` : error.message
}

export const useCustomersStore = defineStore('admin-customers', () => {
  const customers = ref<Customer[]>([])
  const nextCursor = ref<string>()
  const hasMore = ref(false)
  const loading = ref(false)
  const loadingMore = ref(false)
  const saving = ref(false)
  const deletingId = ref<string>()
  const error = ref('')
  let activeRequest: AbortController | undefined

  async function fetchInitial(force = false): Promise<void> {
    if (loading.value || (!force && customers.value.length > 0)) return
    activeRequest?.abort()
    activeRequest = new AbortController()
    loading.value = true
    error.value = ''
    try {
      const response = await customersApi.listCustomers({
        limit: PAGE_SIZE,
        signal: activeRequest.signal,
      })
      customers.value = response.data
      nextCursor.value = response.pagination.next_cursor
      hasMore.value = response.pagination.has_more
    } catch (requestError) {
      if (activeRequest.signal.aborted) return
      error.value = errorMessage(requestError, 'Không thể tải danh sách khách hàng.')
    } finally {
      loading.value = false
    }
  }

  async function fetchNext(): Promise<void> {
    if (loadingMore.value || !hasMore.value || !nextCursor.value) return
    loadingMore.value = true
    error.value = ''
    try {
      const response = await customersApi.listCustomers({
        limit: PAGE_SIZE,
        cursor: nextCursor.value,
      })
      const knownIDs = new Set(customers.value.map((customer) => customer.id))
      customers.value.push(...response.data.filter((customer) => !knownIDs.has(customer.id)))
      nextCursor.value = response.pagination.next_cursor
      hasMore.value = response.pagination.has_more
    } catch (requestError) {
      error.value = errorMessage(requestError, 'Không thể tải thêm khách hàng.')
    } finally {
      loadingMore.value = false
    }
  }

  async function update(id: string, payload: CustomerInput): Promise<Customer> {
    saving.value = true
    error.value = ''
    try {
      const customer = await customersApi.updateCustomer(id, payload)
      const index = customers.value.findIndex((item) => item.id === id)
      if (index >= 0) customers.value[index] = customer
      return customer
    } catch (requestError) {
      error.value = errorMessage(requestError, 'Không thể cập nhật khách hàng.')
      throw requestError
    } finally {
      saving.value = false
    }
  }

  async function remove(id: string): Promise<void> {
    deletingId.value = id
    error.value = ''
    try {
      await customersApi.deleteCustomer(id)
      customers.value = customers.value.filter((customer) => customer.id !== id)
    } catch (requestError) {
      error.value = errorMessage(requestError, 'Không thể xoá tài khoản khách hàng.')
      throw requestError
    } finally {
      deletingId.value = undefined
    }
  }

  return {
    customers,
    hasMore,
    loading,
    loadingMore,
    saving,
    deletingId,
    error,
    fetchInitial,
    fetchNext,
    update,
    remove,
  }
})
