import { beforeEach, describe, expect, it } from 'vitest'
import { consumeCallback, isCodeProvider } from './oauth'
describe('OAuth callback transaction', () => {
  beforeEach(() => sessionStorage.clear())
  const seed = (expiresAt = Date.now() + 60000, redirect = '/sach') =>
    sessionStorage.setItem(
      'bookstore.oauth.discord',
      JSON.stringify({ state: 'expected', expiresAt, redirect }),
    )
  it('accepts a bound callback once', () => {
    seed()
    const query = new URLSearchParams({ state: 'expected', code: 'code' })
    expect(consumeCallback('discord', query).payload.code).toBe('code')
    expect(() => consumeCallback('discord', query)).toThrow()
  })
  it('rejects state mismatch, expiry and denied authorization', () => {
    seed()
    expect(() =>
      consumeCallback('discord', new URLSearchParams({ state: 'wrong', code: 'code' })),
    ).toThrow()
    seed(Date.now() - 1)
    expect(() =>
      consumeCallback('discord', new URLSearchParams({ state: 'expected', code: 'code' })),
    ).toThrow()
    seed()
    expect(() =>
      consumeCallback(
        'discord',
        new URLSearchParams({ state: 'expected', error: 'access_denied' }),
      ),
    ).toThrow()
  })
  it('does not navigate to an external post-login redirect', () => {
    seed(Date.now() + 60000, '//evil.example')
    const result = consumeCallback(
      'discord',
      new URLSearchParams({ state: 'expected', code: 'code' }),
    )
    expect(result.redirect).toBe('/')
    expect(isCodeProvider('google')).toBe(false)
  })
})
