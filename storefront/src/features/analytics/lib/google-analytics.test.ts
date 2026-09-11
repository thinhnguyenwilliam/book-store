import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

async function setup() {
  const analytics = await import('./google-analytics')
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', name: 'home', component: { template: '<div />' } },
      { path: '/sach/:id', name: 'book-detail', component: { template: '<div />' } },
    ],
  })
  await router.push('/')
  analytics.installGoogleAnalytics(router)
  await router.isReady()
  await Promise.resolve()
  return { analytics, router }
}

describe('GA4 consent and data minimization', () => {
  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllEnvs()
  })
  beforeEach(() => {
    vi.resetModules()
    vi.stubEnv('VITE_GA_MEASUREMENT_ID', 'G-TEST123')
    vi.stubEnv('VITE_GA_ENABLE_LOCAL', 'true')
    localStorage.clear()
    document.getElementById('bookstore-ga4')?.remove()
    Object.assign(window, { dataLayer: [], gtag: undefined })
    const append = document.head.appendChild.bind(document.head)
    vi.spyOn(document.head, 'appendChild').mockImplementation((node) => {
      // Inspect tag insertion without executing/downloading Google's real code.
      if (node instanceof HTMLScriptElement) node.type = 'application/json'
      return append(node)
    })
  })

  it('does not load a Google tag before consent or after rejection', async () => {
    const { analytics } = await setup()
    expect(document.getElementById('bookstore-ga4')).toBeNull()
    analytics.setAnalyticsConsent('denied')
    expect(document.getElementById('bookstore-ga4')).toBeNull()
  })

  it('loads once after consent and sends sanitized SPA page views', async () => {
    const { analytics, router } = await setup()
    analytics.setAnalyticsConsent('granted')
    analytics.setAnalyticsConsent('granted')
    await router.push('/sach/secret-id?token=private#private')
    expect(document.querySelectorAll('#bookstore-ga4')).toHaveLength(1)
    const data = (window as Window & { dataLayer: unknown[] }).dataLayer
    const serialized = JSON.stringify(data)
    expect(serialized).not.toContain('private')
    expect(serialized).not.toContain('secret-id')
    expect(serialized).toContain('/sach/:id')
    const calls = data.map((item) => Array.from(item as ArrayLike<unknown>))
    expect(calls.filter((call) => call[0] === 'event' && call[1] === 'page_view')).toHaveLength(2)
  })

  it('does not send free-text search to Google', async () => {
    const { analytics } = await setup()
    analytics.setAnalyticsConsent('granted')
    const data = (window as Window & { dataLayer: unknown[] }).dataLayer
    const before = data.length
    analytics.trackGoogleActivity('book.searched', 'someone@example.com')
    expect(data).toHaveLength(before)
    expect(analytics.analyticsPath('/don-hang/customer-order?token=x')).toBe('/don-hang/:id')
  })

  it('stays disabled without a real measurement ID', async () => {
    vi.stubEnv('VITE_GA_MEASUREMENT_ID', '')
    const { analytics } = await setup()
    analytics.setAnalyticsConsent('granted')
    expect(document.getElementById('bookstore-ga4')).toBeNull()
  })

  it('disables collection and clears its host cookies on withdrawal', async () => {
    const reload = vi.spyOn(window.location, 'reload').mockImplementation(() => undefined)
    const { analytics } = await setup()
    analytics.setAnalyticsConsent('granted')
    document.cookie = '_ga=test; Path=/'
    analytics.setAnalyticsConsent('denied')
    expect(localStorage.getItem('bookstore.ga-consent.v1')).toBe('denied')
    expect(document.cookie).not.toContain('_ga=')
    expect(reload).toHaveBeenCalledOnce()
  })
})
