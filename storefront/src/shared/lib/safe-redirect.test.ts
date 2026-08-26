import { describe, expect, it } from 'vitest'

import { safeRedirectPath } from './safe-redirect'

describe('safeRedirectPath', () => {
  it('allows an internal path with query and hash', () => {
    expect(safeRedirectPath('/tai-khoan?tab=orders#latest', '/')).toBe(
      '/tai-khoan?tab=orders#latest',
    )
  })

  it.each(['https://evil.example', '//evil.example/path', 'javascript:alert(1)', 'account'])(
    'rejects unsafe redirect %s',
    (value) => expect(safeRedirectPath(value, '/tai-khoan')).toBe('/tai-khoan'),
  )
})
