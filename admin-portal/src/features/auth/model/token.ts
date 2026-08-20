import type { JwtClaims } from './types'

export function readJwtClaims(token: string): JwtClaims | null {
  const payload = token.split('.')[1]
  if (!payload) return null

  try {
    const base64 = payload.replace(/-/g, '+').replace(/_/g, '/')
    const padded = base64.padEnd(Math.ceil(base64.length / 4) * 4, '=')
    const bytes = Uint8Array.from(atob(padded), (character) => character.charCodeAt(0))
    return JSON.parse(new TextDecoder().decode(bytes)) as JwtClaims
  } catch {
    return null
  }
}

export function tokenHasRole(token: string, role: string): boolean {
  return readJwtClaims(token)?.roles?.includes(role) ?? false
}
