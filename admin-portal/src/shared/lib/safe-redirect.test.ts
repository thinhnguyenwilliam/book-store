import { describe, expect, it } from 'vitest'

import { safeRedirectPath } from './safe-redirect'

describe('safeRedirectPath', () => {
  it('allows an internal path with query and hash', () => {
    expect(safeRedirectPath('/customers?status=active#latest', '/')).toBe(
      '/customers?status=active#latest',
    )
  })

  it.each(['https://evil.example', '//evil.example/path', 'javascript:alert(1)', 'customers'])(
    'rejects unsafe redirect %s',
    (value) => expect(safeRedirectPath(value, '/')).toBe('/'),
  )
})
