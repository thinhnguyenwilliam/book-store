import { apiRequest } from '@/shared/api/http-client'
import { safeRedirectPath } from '@/shared/lib/safe-redirect'

export type CodeProvider = 'discord' | 'twitter'
export interface OAuthCallback {
  provider: CodeProvider
  code: string
  state: string
  redirect_uri: string
}
interface Attempt {
  state: string
  expiresAt: number
  redirect: string
}
const key = (provider: string) => 'bookstore.oauth.' + provider
export const providerLabel = (provider: CodeProvider) =>
  provider === 'discord' ? 'Discord' : 'X (Twitter)'
export function isCodeProvider(value: unknown): value is CodeProvider {
  return value === 'discord' || value === 'twitter'
}
export function callbackURI(provider: CodeProvider): string {
  return window.location.origin + '/auth/callback/' + provider
}
export async function startOAuth(provider: CodeProvider, redirect: unknown): Promise<void> {
  const response = await apiRequest<{
    state: string
    authorization_url: string
    expires_in: number
  }>('/api/v1/auth/oauth/' + provider + '/start', {
    method: 'POST',
    data: { redirect_uri: callbackURI(provider), create_account: true },
    skipAuthRefresh: true,
  })
  const authorization = new URL(response.authorization_url)
  const expected =
    provider === 'discord'
      ? 'https://discord.com/oauth2/authorize'
      : 'https://x.com/i/oauth2/authorize'
  if (authorization.origin + authorization.pathname !== expected)
    throw new Error('Địa chỉ đăng nhập không hợp lệ.')
  const attempt: Attempt = {
    state: response.state,
    expiresAt: Date.now() + response.expires_in * 1000,
    redirect: safeRedirectPath(redirect, '/tai-khoan'),
  }
  sessionStorage.setItem(key(provider), JSON.stringify(attempt))
  window.location.assign(authorization.toString())
}
export function consumeCallback(
  provider: CodeProvider,
  query: URLSearchParams,
): { payload: OAuthCallback; redirect: string } {
  const raw = sessionStorage.getItem(key(provider))
  sessionStorage.removeItem(key(provider))
  if (!raw) throw new Error('Phiên đăng nhập không tồn tại. Vui lòng bắt đầu lại.')
  const attempt = JSON.parse(raw) as Attempt
  if (!attempt.state || query.get('state') !== attempt.state || Date.now() >= attempt.expiresAt) {
    throw new Error('Phiên đăng nhập đã hết hạn hoặc không hợp lệ. Vui lòng bắt đầu lại.')
  }
  if (query.has('error')) throw new Error('Bạn đã hủy hoặc nhà cung cấp từ chối đăng nhập.')
  const code = query.get('code')
  if (!code || code.length > 4096)
    throw new Error('Không nhận được mã đăng nhập. Vui lòng thử lại.')
  return {
    payload: { provider, code, state: attempt.state, redirect_uri: callbackURI(provider) },
    redirect: safeRedirectPath(attempt.redirect, '/tai-khoan'),
  }
}
