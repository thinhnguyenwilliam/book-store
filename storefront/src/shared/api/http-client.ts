import axios, { AxiosError, type AxiosRequestConfig, type InternalAxiosRequestConfig } from 'axios'

import { env } from '@/shared/config/env'

interface ErrorEnvelope {
  error?: {
    message?: string
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
  ) {
    super(message)
    this.name = 'ApiError'
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
  if (accessToken) {
    config.headers.set('Authorization', `Bearer ${accessToken}`)
  }
  return config
})

api.interceptors.response.use(
  (response) => response,
  async (error: unknown) => {
    if (!axios.isAxiosError(error)) {
      return Promise.reject(toApiError(error))
    }

    const config = error.config as RetryableRequestConfig | undefined
    const shouldRefresh =
      error.response?.status === 401 &&
      Boolean(config) &&
      !config?._retry &&
      !config?.skipAuthRefresh &&
      !isAuthSessionEndpoint(config?.url)

    if (!shouldRefresh || !config) {
      return Promise.reject(toApiError(error))
    }

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
    ['/api/v1/auth/login', '/api/v1/auth/register', '/api/v1/auth/refresh'].some((path) =>
      url.endsWith(path),
    ),
  )
}

function toApiError(error: unknown): ApiError {
  if (error instanceof ApiError) return error
  if (!(error instanceof AxiosError)) {
    return new ApiError(0, 'Đã xảy ra lỗi không xác định. Vui lòng thử lại.')
  }
  if (error.code === AxiosError.ERR_CANCELED) {
    return new ApiError(499, 'Yêu cầu đã bị hủy.')
  }
  if (error.code === AxiosError.ECONNABORTED || error.code === AxiosError.ETIMEDOUT) {
    return new ApiError(408, 'Yêu cầu mất quá nhiều thời gian. Vui lòng thử lại.')
  }
  const payload = error.response?.data as ErrorEnvelope | undefined
  if (error.response) {
    return new ApiError(
      error.response.status,
      payload?.error?.message || `Request failed with status ${error.response.status}`,
    )
  }
  return new ApiError(0, 'Không thể kết nối tới máy chủ. Vui lòng kiểm tra backend.')
}
