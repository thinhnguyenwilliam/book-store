import { apiRequest } from '@/shared/api/http-client'

export interface Role {
  code: string
  name: string
  description: string
  system: boolean
  permissions: string[]
}
export interface Permission {
  code: string
  name: string
  description: string
  group: string
}
export interface Access {
  account_id: string
  roles: string[]
  permissions: string[]
}
export interface Audit {
  id: number
  actor_id: string
  action: string
  target: string
  before: string
  after: string
  trace_id: string
  created_at: string
}
export const catalog = () =>
  apiRequest<{ roles: Role[]; permissions: Permission[] }>('/api/v1/admin/roles')
export const saveRole = (role: Role, create: boolean) =>
  apiRequest<void>(
    create ? '/api/v1/admin/roles' : `/api/v1/admin/roles/${encodeURIComponent(role.code)}`,
    { method: create ? 'POST' : 'PUT', data: role },
  )
export const accountAccess = (id: string) =>
  apiRequest<Access>(`/api/v1/admin/accounts/${encodeURIComponent(id)}/permissions`)
export const assignRoles = (id: string, roles: string[]) =>
  apiRequest<void>(`/api/v1/admin/accounts/${encodeURIComponent(id)}/roles`, {
    method: 'PUT',
    data: { roles },
  })
export const deleteRole = (code: string) =>
  apiRequest<void>(`/api/v1/admin/roles/${encodeURIComponent(code)}`, { method: 'DELETE' })
export const audit = (before?: number) =>
  apiRequest<Audit[]>('/api/v1/admin/authorization/audit', { params: { before_id: before } })
