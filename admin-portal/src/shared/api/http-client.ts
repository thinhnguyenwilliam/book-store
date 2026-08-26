import axios, { AxiosError, type AxiosRequestConfig, type InternalAxiosRequestConfig } from 'axios'

import { env } from '@/shared/config/env'

interface ErrorEnvelope {
  error?: {
    message?: string
    code?: string
    provider?: string
    retryable?: boolean
  }
}

export interface AccessTokenResponse {
  access_token: string
  token_type: string
  user_id: string
  expires_in: number
}

interface RequestConfig extends AxiosRequestConfig {
  skipAuthRefresh?: boolean
}

interface RetryableRequestConfig extends InternalAxiosRequestConfig {
  _retry?: boolean
  skipAuthRefresh?: boolean
}

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
    public readonly traceId?: string,
    public readonly code?: string,
    public readonly provider?: string,
    public readonly retryable = false,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

export function providerLoginErrorMessage(error: unknown, providerLabel: string): string {
  if (!(error instanceof ApiError)) return `Không thể đăng nhập ${providerLabel}.`
  switch (error.code) {
    case 'invalid_oauth_state':
      return `Phiên đăng nhập ${providerLabel} đã hết hạn. Vui lòng thử lại.`
    case 'invalid_provider_credential':
      return `Thông tin đăng nhập ${providerLabel} không hợp lệ hoặc tài khoản chưa được phép.`
    case 'external_identity_conflict':
      return `Email ${providerLabel} đang liên kết với một tài khoản khác.`
    case 'provider_not_configured':
      return `Đăng nhập ${providerLabel} chưa được cấu hình.`
    case 'provider_timeout':
    case 'provider_unavailable':
      return `${providerLabel} đang tạm thời không phản hồi. Vui lòng thử lại.`
    default:
      return error.message || `Không thể đăng nhập ${providerLabel}.`
  }
}

let accessToken: string | null = null
let refreshPromise: Promise<AccessTokenResponse> | null = null
let sessionExpiredHandler: (() => void) | undefined

const api = axios.create({
  baseURL: env.apiBaseUrl,
  timeout: env.apiTimeoutMs,
  withCredentials: true,
  headers: { Accept: 'application/json' },
})

const refreshClient = axios.create({
  baseURL: env.apiBaseUrl,
  timeout: env.apiTimeoutMs,
  withCredentials: true,
  headers: { Accept: 'application/json' },
})

export function setApiAccessToken(token: string | null): void {
  accessToken = token
}

export function onSessionExpired(handler: () => void): void {
  sessionExpiredHandler = handler
}

api.interceptors.request.use((config) => {
  if (accessToken) config.headers.set('Authorization', `Bearer ${accessToken}`)
  return config
})

api.interceptors.response.use(
  (response) => response,
  async (error: unknown) => {
    if (!axios.isAxiosError(error)) return Promise.reject(toApiError(error))

    const config = error.config as RetryableRequestConfig | undefined
    const shouldRefresh =
      error.response?.status === 401 &&
      Boolean(config) &&
      !config?._retry &&
      !config?.skipAuthRefresh &&
      !isAuthSessionEndpoint(config?.url)

    if (!shouldRefresh || !config) return Promise.reject(toApiError(error))

    config._retry = true
    try {
      const session = await refreshAccessToken()
      config.headers.set('Authorization', `Bearer ${session.access_token}`)
      return await api.request(config)
    } catch (refreshError) {
      return Promise.reject(toApiError(refreshError))
    }
  },
)

export async function apiRequest<T>(url: string, config: RequestConfig = {}): Promise<T> {
  try {
    const response = await api.request<T>({ ...config, url })
    return response.data
  } catch (error) {
    throw toApiError(error)
  }
}

export function refreshApiSession(): Promise<AccessTokenResponse> {
  return refreshAccessToken()
}

function refreshAccessToken(): Promise<AccessTokenResponse> {
  if (!refreshPromise) {
    refreshPromise = withRefreshLock(async () => {
      const { data } = await refreshClient.post<AccessTokenResponse>('/api/v1/auth/refresh')
      setApiAccessToken(data.access_token)
      return data
    })
      .catch((error: unknown) => {
        setApiAccessToken(null)
        sessionExpiredHandler?.()
        throw toApiError(error)
      })
      .finally(() => {
        refreshPromise = null
      })
  }
  return refreshPromise
}

function withRefreshLock<T>(operation: () => Promise<T>): Promise<T> {
  if (typeof navigator !== 'undefined' && navigator.locks) {
    return navigator.locks.request('bookstore-auth-refresh', { mode: 'exclusive' }, operation)
  }
  return operation()
}

function isAuthSessionEndpoint(url?: string): boolean {
  return Boolean(
    url &&
    [
      '/api/v1/auth/login',
      '/api/v1/auth/google',
      '/api/v1/auth/facebook',
      '/api/v1/auth/refresh',
    ].some((path) => url.endsWith(path)),
  )
}

function toApiError(error: unknown): ApiError {
  if (error instanceof ApiError) return error
  if (!(error instanceof AxiosError)) return new ApiError(0, 'Đã xảy ra lỗi không xác định.')
  const traceId = error.response?.headers['x-trace-id'] as string | undefined
  if (error.code === AxiosError.ERR_CANCELED)
    return new ApiError(499, 'Yêu cầu đã bị hủy.', traceId)
  if (error.code === AxiosError.ECONNABORTED || error.code === AxiosError.ETIMEDOUT) {
    return new ApiError(408, 'Yêu cầu mất quá nhiều thời gian.', traceId)
  }
  const payload = error.response?.data as ErrorEnvelope | undefined
  if (error.response) {
    return new ApiError(
      error.response.status,
      payload?.error?.message || `Request failed with status ${error.response.status}`,
      traceId,
      payload?.error?.code,
      payload?.error?.provider,
      payload?.error?.retryable ?? false,
    )
  }
  return new ApiError(0, 'Không thể kết nối tới backend.', traceId)
}
