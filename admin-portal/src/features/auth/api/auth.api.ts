import { apiRequest, refreshApiSession } from '@/shared/api/http-client'

import type {
  AuthResponse,
  FacebookLoginPayload,
  GoogleLoginPayload,
  LoginPayload,
  ProviderStatePayload,
  ProviderStateResponse,
  UserProfile,
} from '../model/types'

export function login(payload: LoginPayload) {
  return apiRequest<AuthResponse>('/api/v1/auth/login', { method: 'POST', data: payload })
}

export function createProviderState(payload: ProviderStatePayload) {
  return apiRequest<ProviderStateResponse>('/api/v1/auth/provider-state', {
    method: 'POST',
    data: payload,
    skipAuthRefresh: true,
  })
}

export function loginWithGoogle(payload: GoogleLoginPayload) {
  return apiRequest<AuthResponse>('/api/v1/auth/google', { method: 'POST', data: payload })
}

export function loginWithFacebook(payload: FacebookLoginPayload) {
  return apiRequest<AuthResponse>('/api/v1/auth/facebook', { method: 'POST', data: payload })
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
