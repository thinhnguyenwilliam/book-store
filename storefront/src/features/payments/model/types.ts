export interface Payment {
  id: string
  order_id: string
  status: 'pending' | 'succeeded' | 'failed' | 'refund_pending' | 'refunded'
  amount_cents: number
  platform_fee_cents: number
  currency: string
  failure_reason: string
  provider: string
  provider_transaction_id: string
  checkout_url: string
  expires_at?: string
  paid_at?: string
  created_at: string
  updated_at: string
}
