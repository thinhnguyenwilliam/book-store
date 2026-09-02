function required(name: keyof ImportMetaEnv): string {
  const value = import.meta.env[name]?.trim()
  if (!value) {
    throw new Error(`Missing required environment variable: ${name}`)
  }
  return value
}

function positiveNumber(value: string | undefined, fallback: number): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback
}

function booleanValue(value: string | undefined): boolean {
  return value?.trim().toLowerCase() === 'true'
}

export const env = Object.freeze({
  apiBaseUrl: required('VITE_API_BASE_URL').replace(/\/$/, ''),
  apiTimeoutMs: positiveNumber(import.meta.env.VITE_API_TIMEOUT_MS, 10_000),
  vnpayEnabled: booleanValue(import.meta.env.VITE_VNPAY_ENABLED),
  googleClientId: import.meta.env.VITE_GOOGLE_CLIENT_ID?.trim() || '',
  facebookAppId: import.meta.env.VITE_FACEBOOK_APP_ID?.trim() || '',
  facebookGraphVersion: import.meta.env.VITE_FACEBOOK_GRAPH_VERSION?.trim() || 'v25.0',
  firebase: Object.freeze({
    apiKey: import.meta.env.VITE_FIREBASE_API_KEY?.trim() || '',
    authDomain: import.meta.env.VITE_FIREBASE_AUTH_DOMAIN?.trim() || '',
    projectId: import.meta.env.VITE_FIREBASE_PROJECT_ID?.trim() || '',
    storageBucket: import.meta.env.VITE_FIREBASE_STORAGE_BUCKET?.trim() || '',
    messagingSenderId: import.meta.env.VITE_FIREBASE_MESSAGING_SENDER_ID?.trim() || '',
    appId: import.meta.env.VITE_FIREBASE_APP_ID?.trim() || '',
    vapidKey: import.meta.env.VITE_FIREBASE_VAPID_KEY?.trim() || '',
  }),
})
