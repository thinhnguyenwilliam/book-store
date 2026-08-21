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

export const env = Object.freeze({
  apiBaseUrl: required('VITE_API_BASE_URL').replace(/\/$/, ''),
  apiTimeoutMs: positiveNumber(import.meta.env.VITE_API_TIMEOUT_MS, 10_000),
  googleClientId: import.meta.env.VITE_GOOGLE_CLIENT_ID?.trim() || '',
  facebookAppId: import.meta.env.VITE_FACEBOOK_APP_ID?.trim() || '',
  facebookGraphVersion: import.meta.env.VITE_FACEBOOK_GRAPH_VERSION?.trim() || 'v25.0',
})
