export interface Customer {
  id: string
  email: string
  display_name: string
  created_at: string
  updated_at: string
}

export interface CustomerInput {
  display_name: string
}

export interface CustomerListResponse {
  data: Customer[]
  pagination: {
    next_cursor?: string
    has_more: boolean
  }
}
