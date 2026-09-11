<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/features/auth/model/auth.store'
import { consumeCallback, isCodeProvider, providerLabel } from '@/features/auth/lib/oauth'
import { providerLoginErrorMessage } from '@/shared/api/http-client'
const router = useRouter()
const auth = useAuthStore()
const error = ref('')
onMounted(async () => {
  const provider = window.location.pathname.split('/').pop()
  const query = new URLSearchParams(window.location.search)
  // Remove authorization codes/errors from visible history before any further navigation.
  window.history.replaceState(window.history.state, '', window.location.pathname)
  if (!isCodeProvider(provider)) {
    error.value = 'Nhà cung cấp không hợp lệ.'
    return
  }
  try {
    const result = consumeCallback(provider, query)
    await auth.signInWithOAuth(result.payload)
    await router.replace(result.redirect)
  } catch (cause) {
    error.value =
      cause instanceof Error && !('status' in cause)
        ? cause.message
        : providerLoginErrorMessage(cause, providerLabel(provider))
  }
})
</script>
<template>
  <main class="oauth-callback">
    <h1>{{ error ? 'Chưa thể đăng nhập' : 'Đang hoàn tất đăng nhập…' }}</h1>
    <p v-if="error" role="alert">{{ error }}</p>
    <RouterLink v-if="error" :to="{ name: 'login' }">Quay lại đăng nhập</RouterLink>
  </main>
</template>
<style scoped>
.oauth-callback {
  max-width: 36rem;
  margin: 12vh auto;
  padding: 2rem;
  line-height: 1.6;
}
</style>
