import { graphQLRequest } from '@/shared/api/graphql-client'

export interface OrderItem {
  id: string
  bookId: string
  sellerId: string
  title: string
  unitPriceCents: number
  quantity: number
  subtotalCents: number
}

export interface OrderDetail {
  order: {
    id: string
    status: string
    totalCents: number
    currency: string
    items: OrderItem[]
    failureReason?: string
    reservationExpiresAt?: string
    createdAt: string
    updatedAt: string
  }
  payment?: {
    id: string
    status: string
    amountCents: number
    platformFeeCents: number
    provider: string
    providerTransactionId?: string
    checkoutUrl?: string
    paidAt?: string
  }
}

interface OrderDetailResult {
  orderDetail: OrderDetail
}

const ORDER_DETAIL_QUERY = `
  query OrderDetail($id: ID!) {
    orderDetail(id: $id) {
      order {
        id status totalCents currency failureReason reservationExpiresAt createdAt updatedAt
        items { id bookId sellerId title unitPriceCents quantity subtotalCents }
      }
      payment {
        id status amountCents platformFeeCents provider providerTransactionId checkoutUrl paidAt
      }
    }
  }
`

export async function getOrderDetail(id: string, signal?: AbortSignal): Promise<OrderDetail> {
  const response = await graphQLRequest<OrderDetailResult>(ORDER_DETAIL_QUERY, { id }, signal)
  return response.orderDetail
}
