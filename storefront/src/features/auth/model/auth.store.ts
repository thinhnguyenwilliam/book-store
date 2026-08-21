import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { ApiError, onSessionExpired, setApiAccessToken } from '@/shared/api/http-client'
import { disableGoogleAutoSelect } from '@/shared/lib/google-identity'
import * as authApi from '../api/auth.api'
import type { LoginPayload, RegisterPayload, UserProfile } from './types'

function sleep(duration: number): Promise<void> {
  return new Promise((resolve) => window.setTimeout(resolve, duration))
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(null)
  const profile = ref<UserProfile | null>(null)
  const loading = ref(false)
  const profileLoading = ref(false)
  const initialized = ref(false)

  const isAuthenticated = computed(() => Boolean(token.value))
  const displayName = computed(
    () => profile.value?.display_name || profile.value?.email || 'Độc giả',
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

  async function fetchProfile(): Promise<void> {
    if (!isAuthenticated.value) return
    profileLoading.value = true
    try {
      profile.value = await authApi.getProfile()
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        clearSession()
      }
      throw error
    } finally {
      profileLoading.value = false
    }
  }

  async function initialize(): Promise<void> {
    if (initialized.value) return
    try {
      const response = await authApi.refreshSession()
      applyAccessToken(response.access_token)
      await fetchProfile()
    } catch (error) {
      if (error instanceof ApiError && (error.status === 401 || error.status === 403)) {
        clearSession()
      }
      // An unavailable backend must not prevent public routes from rendering.
    } finally {
      initialized.value = true
    }
  }

  async function signIn(payload: LoginPayload): Promise<void> {
    loading.value = true
    try {
      const response = await authApi.login(payload)
      applyAccessToken(response.access_token)
      await fetchProfile()
    } finally {
      loading.value = false
    }
  }

  async function signUp(payload: RegisterPayload): Promise<void> {
    loading.value = true
    try {
      const response = await authApi.register(payload)
      applyAccessToken(response.access_token)

      await fetchProfileWithRetry()
    } finally {
      loading.value = false
    }
  }

  async function signInWithGoogle(credential: string): Promise<void> {
    loading.value = true
    try {
      const response = await authApi.loginWithGoogle({ credential, create_account: true })
      applyAccessToken(response.access_token)
      await fetchProfileWithRetry()
    } finally {
      loading.value = false
    }
  }

  async function signInWithFacebook(accessToken: string): Promise<void> {
    loading.value = true
    try {
      const response = await authApi.loginWithFacebook({
        access_token: accessToken,
        create_account: true,
      })
      applyAccessToken(response.access_token)
      await fetchProfileWithRetry()
    } finally {
      loading.value = false
    }
  }

  async function fetchProfileWithRetry(): Promise<void> {
    // New profiles are created asynchronously through the outbox and RabbitMQ worker.
    for (let attempt = 0; attempt < 5; attempt += 1) {
      try {
        await fetchProfile()
        return
      } catch (error) {
        if (!(error instanceof ApiError) || error.status !== 404) {
          throw error
        }
        if (attempt < 4) {
          await sleep(400 * (attempt + 1))
        }
      }
    }
  }

  async function saveDisplayName(value: string): Promise<void> {
    profile.value = await authApi.updateProfile(value)
  }

  async function signOut(): Promise<void> {
    try {
      await authApi.logout()
    } finally {
      disableGoogleAutoSelect()
      clearSession()
    }
  }

  return {
    profile,
    loading,
    profileLoading,
    initialized,
    isAuthenticated,
    displayName,
    initialize,
    fetchProfile,
    signIn,
    signUp,
    signInWithGoogle,
    signInWithFacebook,
    saveDisplayName,
    signOut,
  }
})
