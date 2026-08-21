import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { ApiError, onSessionExpired, setApiAccessToken } from '@/shared/api/http-client'
import { disableGoogleAutoSelect } from '@/shared/lib/google-identity'
import * as authApi from '../api/auth.api'
import { tokenHasRole } from './token'
import type { LoginPayload, UserProfile } from './types'

export const useAuthStore = defineStore('admin-auth', () => {
  const token = ref<string | null>(null)
  const profile = ref<UserProfile | null>(null)
  const loading = ref(false)
  const initialized = ref(false)
  const accessDenied = ref(false)

  const isAuthenticated = computed(() => Boolean(token.value))
  const isAdmin = computed(() => Boolean(token.value && tokenHasRole(token.value, 'admin')))
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
    setApiAccessToken(null)
  }

  onSessionExpired(clearSession)

  async function establishAdminSession(accessToken: string): Promise<void> {
    applyAccessToken(accessToken)
    if (!tokenHasRole(accessToken, 'admin')) {
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

  async function signInWithGoogle(credential: string): Promise<void> {
    loading.value = true
    try {
      const response = await authApi.loginWithGoogle({ credential, create_account: false })
      await establishAdminSession(response.access_token)
    } finally {
      loading.value = false
    }
  }

  async function signInWithFacebook(accessToken: string): Promise<void> {
    loading.value = true
    try {
      const response = await authApi.loginWithFacebook({
        access_token: accessToken,
        create_account: false,
      })
      await establishAdminSession(response.access_token)
    } finally {
      loading.value = false
    }
  }

  async function signOut(): Promise<void> {
    try {
      await authApi.logout()
    } finally {
      accessDenied.value = false
      disableGoogleAutoSelect()
      clearSession()
    }
  }

  return {
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
    signOut,
  }
})
