import { apiRequest, refreshApiSession } from '@/shared/api/http-client'

import type { AuthResponse, LoginPayload, UserProfile } from '../model/types'

export function login(payload: LoginPayload) {
  return apiRequest<AuthResponse>('/api/v1/auth/login', { method: 'POST', data: payload })
}

export function refreshSession() {
  return refreshApiSession()
}

export function logout() {
  return apiRequest<void>('/api/v1/auth/logout', { method: 'POST', skipAuthRefresh: true })
}

export function getProfile() {
  return apiRequest<UserProfile>('/api/v1/users/me')
}
