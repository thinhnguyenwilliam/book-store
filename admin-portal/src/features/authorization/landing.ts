export function adminLandingRoute(can: (permission: string) => boolean): string {
  if (can('analytics.read') && can('books.read') && can('customers.read')) return 'dashboard'
  if (can('books.read')) return 'books'
  if (can('customers.read')) return 'customers'
  if (can('chat.read')) return 'chat'
  if (can('roles.read')) return 'authorization'
  return 'forbidden'
}

export function canGrantRole(
  role: { code: string; permissions: string[] },
  actorRoles: string[],
  actorPermissions: string[],
): boolean {
  if (role.code === 'super_admin' && !actorRoles.includes('super_admin')) return false
  return role.permissions.every((permission) => actorPermissions.includes(permission))
}
