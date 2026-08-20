import { describe, expect, it } from 'vitest'

import { formatPrice, initials } from './format'

describe('format helpers', () => {
  it('converts backend cents into a readable USD price', () => {
    expect(formatPrice(3999)).toContain('39,99')
  })

  it('uses the last two words for initials', () => {
    expect(initials('Nguyễn Văn An')).toBe('VA')
  })
})
