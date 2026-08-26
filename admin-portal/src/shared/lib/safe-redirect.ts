const redirectBase = 'https://bookstore.invalid'

export function safeRedirectPath(value: unknown, fallback: string): string {
  if (typeof value !== 'string' || !value.startsWith('/') || value.startsWith('//')) {
    return fallback
  }
  try {
    const parsed = new URL(value, redirectBase)
    if (parsed.origin !== redirectBase || parsed.username || parsed.password) return fallback
    return `${parsed.pathname}${parsed.search}${parsed.hash}`
  } catch {
    return fallback
  }
}
