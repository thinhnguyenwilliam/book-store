import { describe, expect, it } from 'vitest'

import { formatPrice, initials } from './format'

describe('admin format helpers', () => {
  it('formats backend cents as USD', () => {
    expect(formatPrice(3999)).toContain('39,99')
  })

  it('creates compact initials', () => {
    expect(initials('Thịnh Nguyễn')).toBe('TN')
  })
})
