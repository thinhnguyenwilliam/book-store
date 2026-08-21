/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_BASE_URL: string
  readonly VITE_API_TIMEOUT_MS?: string
  readonly VITE_STOREFRONT_URL: string
  readonly VITE_GOOGLE_CLIENT_ID?: string
  readonly VITE_FACEBOOK_APP_ID?: string
  readonly VITE_FACEBOOK_GRAPH_VERSION?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
