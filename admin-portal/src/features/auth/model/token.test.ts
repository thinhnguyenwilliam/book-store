import { describe, expect, it } from 'vitest'

import { readJwtClaims, tokenHasRole } from './token'

function tokenWithPayload(payload: object): string {
  const encoded = btoa(JSON.stringify(payload))
    .replace(/=/g, '')
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
  return `header.${encoded}.signature`
}

describe('JWT role helpers', () => {
  it('reads admin role without treating the client check as authorization', () => {
    const token = tokenWithPayload({ sub: 'user-1', roles: ['customer', 'admin'] })
    expect(readJwtClaims(token)?.sub).toBe('user-1')
    expect(tokenHasRole(token, 'admin')).toBe(true)
  })

  it('rejects malformed and non-admin tokens', () => {
    expect(tokenHasRole('malformed', 'admin')).toBe(false)
    expect(tokenHasRole(tokenWithPayload({ roles: ['customer'] }), 'admin')).toBe(false)
  })
})
