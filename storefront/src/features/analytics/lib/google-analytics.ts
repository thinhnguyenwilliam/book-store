import { ref } from 'vue'
import type { Router } from 'vue-router'

const CONSENT_KEY = 'bookstore.ga-consent.v1'
const measurementID = import.meta.env.VITE_GA_MEASUREMENT_ID?.trim() || ''
const localHost = ['localhost', '127.0.0.1', '[::1]'].includes(window.location.hostname)
export const analyticsConfigured =
  /^G-[A-Z0-9]+$/.test(measurementID) &&
  (!import.meta.env.DEV || import.meta.env.VITE_GA_ENABLE_LOCAL === 'true') &&
  (!localHost || import.meta.env.VITE_GA_ENABLE_LOCAL === 'true')

export type AnalyticsConsent = 'granted' | 'denied' | 'unset'
function readConsent(): AnalyticsConsent {
  try {
    const value = localStorage.getItem(CONSENT_KEY)
    return value === 'granted' || value === 'denied' ? value : 'unset'
  } catch {
    return 'unset'
  }
}
export const analyticsConsent = ref<AnalyticsConsent>(readConsent())
type GtagWindow = Window & {
  dataLayer?: unknown[]
  gtag?: (...args: unknown[]) => void
}
const gaWindow = window as GtagWindow
let router: Router | undefined
let started = false
let lastPage = ''

function referrerOrigin(): string {
  try {
    const url = new URL(document.referrer)
    return ['http:', 'https:'].includes(url.protocol) ? url.origin : ''
  } catch {
    return ''
  }
}

// Use route templates, never raw URLs/query strings containing tokens, search
// text, customer IDs, payment gateway signatures or order IDs.
export function analyticsPath(path: string): string {
  const clean = path.split(/[?#]/)[0] || '/'
  if (/^\/sach\/[^/]+$/.test(clean)) return '/sach/:id'
  if (/^\/don-hang\/[^/]+$/.test(clean)) return '/don-hang/:id'
  return [
    '/',
    '/sach',
    '/gio-hang',
    '/dang-nhap',
    '/dang-ky',
    '/tai-khoan',
    '/thanh-toan/ket-qua',
  ].includes(clean)
    ? clean
    : '/not-found'
}

function pageView(): void {
  if (!started || analyticsConsent.value !== 'granted' || !router) return
  const path = analyticsPath(router.currentRoute.value.path)
  if (lastPage === router.currentRoute.value.path) return
  const previousPage = lastPage
    ? window.location.origin + analyticsPath(lastPage)
    : referrerOrigin()
  lastPage = router.currentRoute.value.path
  gaWindow.gtag?.('event', 'page_view', {
    page_location: window.location.origin + path,
    page_title: String(router.currentRoute.value.name || 'Book Store'),
    page_referrer: previousPage,
  })
}

function start(): void {
  if (!analyticsConfigured || analyticsConsent.value !== 'granted') return
  if (!started) {
    started = true
    gaWindow.dataLayer ||= []
    // gtag expects the arguments object, not a nested array.
    gaWindow.gtag = function () {
      // eslint-disable-next-line prefer-rest-params -- Google's gtag queue consumes an arguments object.
      gaWindow.dataLayer?.push(arguments)
    }
    gaWindow.gtag('consent', 'default', {
      analytics_storage: 'granted',
      ad_storage: 'denied',
      ad_user_data: 'denied',
      ad_personalization: 'denied',
    })
    gaWindow.gtag('js', new Date())
    gaWindow.gtag('config', measurementID, {
      send_page_view: false,
      allow_google_signals: false,
      allow_ad_personalization_signals: false,
      cookie_domain: 'none',
      page_location: window.location.origin + analyticsPath(router?.currentRoute.value.path || '/'),
      page_referrer: referrerOrigin(),
    })
    const script = document.createElement('script')
    script.id = 'bookstore-ga4'
    script.async = true
    script.src = 'https://www.googletagmanager.com/gtag/js?id=' + encodeURIComponent(measurementID)
    document.head.appendChild(script)
  }
  pageView()
}

export function setAnalyticsConsent(value: 'granted' | 'denied'): void {
  analyticsConsent.value = value
  try {
    localStorage.setItem(CONSENT_KEY, value)
  } catch {
    /* storage is optional */
  }
  if (value === 'granted') {
    start()
  } else if (started) {
    // Stop the loaded tag immediately, then reload into basic consent mode.
    Object.assign(window, { ['ga-disable-' + measurementID]: true })
    for (const cookie of document.cookie.split(';')) {
      const name = cookie.split('=')[0]?.trim()
      if (name === '_ga' || name?.startsWith('_ga_')) {
        document.cookie =
          name + '=; Max-Age=0; Expires=Thu, 01 Jan 1970 00:00:00 GMT; Path=/; SameSite=Lax'
      }
    }
    window.location.reload()
  }
}

export function installGoogleAnalytics(appRouter: Router): void {
  router = appRouter
  router.afterEach((_to, _from, failure) => {
    if (!failure) pageView()
  })
  void router.isReady().then(start)
}

export function trackGoogleActivity(event: string, bookID?: string, quantity?: number): void {
  if (!started || analyticsConsent.value !== 'granted') return
  const events: Record<string, string> = {
    'book.viewed': 'view_item',
    'book.added_to_cart': 'add_to_cart',
    'book.removed_from_cart': 'remove_from_cart',
    'checkout.started': 'begin_checkout',
  }
  const name = events[event]
  if (!name) return
  const params: Record<string, unknown> = {
    page_location: window.location.origin + analyticsPath(router?.currentRoute.value.path || '/'),
    page_referrer: '',
  }
  if (bookID && /^[a-f0-9-]{36}$/i.test(bookID)) {
    params.items = [{ item_id: bookID, quantity: quantity || 1 }]
  }
  gaWindow.gtag?.('event', name, params)
}
