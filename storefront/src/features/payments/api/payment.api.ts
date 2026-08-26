import { apiRequest } from '@/shared/api/http-client'
import type { Payment } from '../model/types'

export function getPayment(paymentId: string): Promise<Payment> {
  return apiRequest<Payment>(`/api/v1/payments/${encodeURIComponent(paymentId)}`)
}
