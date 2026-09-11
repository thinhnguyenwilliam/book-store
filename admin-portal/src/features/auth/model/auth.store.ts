import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { ApiError, onSessionExpired, setApiAccessToken } from '@/shared/api/http-client'
import { disableGoogleAutoSelect } from '@/shared/lib/google-identity'
import { revokeLocalPushToken, unregisterCurrentPushDevice } from '@/features/push/lib/push'
import * as authApi from '../api/auth.api'
import type { OAuthCallback } from '../lib/oauth'
import { apiRequest } from '@/shared/api/http-client'
import type { AuthResponse } from './types'
import type { LoginPayload, UserProfile } from './types'

export const useAuthStore = defineStore('admin-auth', () => {
  const token = ref<string | null>(null)
  const profile = ref<UserProfile | null>(null)
  const loading = ref(false)
  const initialized = ref(false)
  const accessDenied = ref(false)
  const permissions = ref<string[]>([])
  const roles = ref<string[]>([])
  const can = (permission: string): boolean => permissions.value.includes(permission)
  async function refreshPermissions(): Promise<void> {
    try {
      const result = await apiRequest<{ roles: string[]; permissions: string[] }>(
        '/api/v1/auth/me/permissions',
      )
      permissions.value = result.permissions
      roles.value = result.roles
    } catch (error) {
      permissions.value = []
      roles.value = []
      throw error
    }
  }

  const isAuthenticated = computed(() => Boolean(token.value))
  const isAdmin = computed(() => Boolean(token.value && can('admin.access')))
  const displayName = computed(
    () => profile.value?.display_name || profile.value?.email || 'Quản trị viên',
  )

  function applyAccessToken(accessToken: string): void {
    token.value = accessToken
    setApiAccessToken(accessToken)
  }

  function clearSession(): void {
    token.value = null
    profile.value = null
    permissions.value = []
    roles.value = []
    setApiAccessToken(null)
  }

  onSessionExpired(() => {
    void revokeLocalPushToken()
    clearSession()
  })

  async function establishAdminSession(accessToken: string): Promise<void> {
    applyAccessToken(accessToken)
    await refreshPermissions()
    if (!can('admin.access')) {
      accessDenied.value = true
      try {
        await authApi.logout()
      } catch {
        // The local session must still be cleared when backend logout is unavailable.
      } finally {
        clearSession()
      }
      throw new ApiError(403, 'Tài khoản này không có quyền quản trị.')
    }
    accessDenied.value = false
    profile.value = await authApi.getProfile()
  }

  async function initialize(): Promise<void> {
    if (initialized.value) return
    try {
      const response = await authApi.refreshSession()
      await establishAdminSession(response.access_token)
    } catch (error) {
      if (!(error instanceof ApiError) || error.status !== 403) accessDenied.value = false
      clearSession()
    } finally {
      initialized.value = true
    }
  }

  async function signIn(payload: LoginPayload): Promise<void> {
    loading.value = true
    try {
      const response = await authApi.login(payload)
      await establishAdminSession(response.access_token)
    } finally {
      loading.value = false
    }
  }

  async function signInWithGoogle(credential: string, state: string): Promise<void> {
    loading.value = true
    try {
      const response = await authApi.loginWithGoogle({ credential, state, create_account: false })
      await establishAdminSession(response.access_token)
    } finally {
      loading.value = false
    }
  }

  async function signInWithFacebook(accessToken: string, state: string): Promise<void> {
    loading.value = true
    try {
      const response = await authApi.loginWithFacebook({
        access_token: accessToken,
        state,
        create_account: false,
      })
      await establishAdminSession(response.access_token)
    } finally {
      loading.value = false
    }
  }

  async function signInWithOAuth(payload: OAuthCallback): Promise<void> {
    loading.value = true
    try {
      const response = await apiRequest<AuthResponse>(
        '/api/v1/auth/oauth/' + payload.provider + '/finish',
        {
          method: 'POST',
          data: { ...payload, create_account: false },
          skipAuthRefresh: true,
        },
      )
      await establishAdminSession(response.access_token)
    } finally {
      loading.value = false
    }
  }

  async function signOut(): Promise<void> {
    try {
      await unregisterCurrentPushDevice()
      await authApi.logout()
    } finally {
      accessDenied.value = false
      disableGoogleAutoSelect()
      clearSession()
    }
  }

  return {
    permissions,
    roles,
    can,
    refreshPermissions,
    profile,
    loading,
    initialized,
    accessDenied,
    isAuthenticated,
    isAdmin,
    displayName,
    initialize,
    signIn,
    signInWithGoogle,
    signInWithFacebook,
    signInWithOAuth,
    signOut,
  }
})
