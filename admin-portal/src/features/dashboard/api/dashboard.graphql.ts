import { graphQLRequest } from '@/shared/api/graphql-client'
import type { Book } from '@/features/books/model/types'
import type { Customer } from '@/features/customers/model/types'

interface GraphQLBook {
  id: string
  title: string
  author: string
  isbn: string
  priceCents: number
  stock: number
  createdAt: string
  updatedAt: string
}

interface GraphQLCustomer {
  id: string
  email: string
  displayName: string
  createdAt: string
  updatedAt: string
}

interface DashboardResult {
  adminDashboard: {
    catalog: {
      loadedCount: number
      inventoryUnits: number
      lowStockCount: number
      inventoryValueCents: number
      recentBooks: GraphQLBook[]
      lowStockBooks: GraphQLBook[]
      pageInfo: { hasNextPage: boolean }
    }
    customers: {
      loadedCount: number
      recentCustomers: GraphQLCustomer[]
      pageInfo: { hasNextPage: boolean }
    }
  }
}

export interface DashboardSnapshot {
  loadedCount: number
  inventoryUnits: number
  lowStockCount: number
  inventoryValueCents: number
  recentBooks: Book[]
  lowStockBooks: Book[]
  customers: Customer[]
  hasMoreBooks: boolean
  hasMoreCustomers: boolean
}

const DASHBOARD_QUERY = `
  query AdminDashboard($booksFirst: Int!, $customersFirst: Int!) {
    adminDashboard(booksFirst: $booksFirst, customersFirst: $customersFirst) {
      catalog {
        loadedCount inventoryUnits lowStockCount inventoryValueCents
        recentBooks { id title author isbn priceCents stock createdAt updatedAt }
        lowStockBooks { id title author isbn priceCents stock createdAt updatedAt }
        pageInfo { hasNextPage }
      }
      customers {
        loadedCount
        recentCustomers { id email displayName createdAt updatedAt }
        pageInfo { hasNextPage }
      }
    }
  }
`

function mapBook(item: GraphQLBook): Book {
  return {
    id: item.id,
    title: item.title,
    author: item.author,
    isbn: item.isbn,
    price_cents: item.priceCents,
    stock: item.stock,
    created_at: item.createdAt,
    updated_at: item.updatedAt,
  }
}

export async function getAdminDashboard(signal?: AbortSignal): Promise<DashboardSnapshot> {
  const { adminDashboard } = await graphQLRequest<DashboardResult>(
    DASHBOARD_QUERY,
    { booksFirst: 30, customersFirst: 5 },
    signal,
  )
  return {
    loadedCount: adminDashboard.catalog.loadedCount,
    inventoryUnits: adminDashboard.catalog.inventoryUnits,
    lowStockCount: adminDashboard.catalog.lowStockCount,
    inventoryValueCents: adminDashboard.catalog.inventoryValueCents,
    recentBooks: adminDashboard.catalog.recentBooks.map(mapBook),
    lowStockBooks: adminDashboard.catalog.lowStockBooks.map(mapBook),
    customers: adminDashboard.customers.recentCustomers.map((item) => ({
      id: item.id,
      email: item.email,
      display_name: item.displayName,
      created_at: item.createdAt,
      updated_at: item.updatedAt,
    })),
    hasMoreBooks: adminDashboard.catalog.pageInfo.hasNextPage,
    hasMoreCustomers: adminDashboard.customers.pageInfo.hasNextPage,
  }
}
