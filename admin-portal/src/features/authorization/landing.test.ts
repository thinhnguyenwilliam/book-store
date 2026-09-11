import { describe, expect, it } from 'vitest'

import { adminLandingRoute, canGrantRole } from './landing'

describe('adminLandingRoute', () => {
  it('sends catalog managers to books instead of the full dashboard', () => {
    const can = (permission: string) =>
      ['admin.access', 'books.read', 'books.create'].includes(permission)
    expect(adminLandingRoute(can)).toBe('books')
  })

  it('keeps operators with catalog, customer and analytics access on the dashboard', () => {
    const can = (permission: string) =>
      ['analytics.read', 'books.read', 'customers.read'].includes(permission)
    expect(adminLandingRoute(can)).toBe('dashboard')
  })
})

describe('canGrantRole', () => {
  it('blocks super_admin unless the actor already has that role', () => {
    const role = { code: 'super_admin', permissions: ['admin.access', 'roles.manage'] }
    expect(canGrantRole(role, ['admin'], role.permissions)).toBe(false)
    expect(canGrantRole(role, ['super_admin'], role.permissions)).toBe(true)
  })

  it('forbids granting a role whose permissions exceed the actor', () => {
    expect(
      canGrantRole(
        { code: 'admin', permissions: ['admin.access', 'books.delete'] },
        ['admin'],
        ['admin.access', 'books.read'],
      ),
    ).toBe(false)
  })
})
