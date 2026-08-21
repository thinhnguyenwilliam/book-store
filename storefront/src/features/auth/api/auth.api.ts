import { apiRequest, refreshApiSession } from '@/shared/api/http-client'

import type {
  AuthResponse,
  GoogleLoginPayload,
  LoginPayload,
  RegisterPayload,
  UserProfile,
} from '../model/types'

export function login(payload: LoginPayload) {
  return apiRequest<AuthResponse>('/api/v1/auth/login', { method: 'POST', data: payload })
}

export function register(payload: RegisterPayload) {
  return apiRequest<AuthResponse>('/api/v1/auth/register', { method: 'POST', data: payload })
}

export function loginWithGoogle(payload: GoogleLoginPayload) {
  return apiRequest<AuthResponse>('/api/v1/auth/google', { method: 'POST', data: payload })
}

export function refreshSession() {
  return refreshApiSession()
}

export function logout() {
  return apiRequest<void>('/api/v1/auth/logout', {
    method: 'POST',
    skipAuthRefresh: true,
  })
}

export function getProfile() {
  return apiRequest<UserProfile>('/api/v1/users/me')
}

export function updateProfile(displayName: string) {
  return apiRequest<UserProfile>('/api/v1/users/me', {
    method: 'PUT',
    data: { display_name: displayName },
  })
}
