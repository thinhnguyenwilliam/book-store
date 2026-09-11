<script setup lang="ts">
import { ref } from 'vue'
import { useRoute } from 'vue-router'
import { startOAuth, providerLabel, type CodeProvider } from '../lib/oauth'
import { providerLoginErrorMessage } from '@/shared/api/http-client'
const props = defineProps<{ provider: CodeProvider }>()
const route = useRoute()
const loading = ref(false)
const error = ref('')
const enabled =
  props.provider === 'discord'
    ? import.meta.env.VITE_DISCORD_ENABLED === 'true'
    : import.meta.env.VITE_TWITTER_ENABLED === 'true'
async function signIn(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    await startOAuth(props.provider, route.query.redirect)
  } catch (cause) {
    error.value = providerLoginErrorMessage(cause, providerLabel(props.provider))
  } finally {
    loading.value = false
  }
}
</script>
<template>
  <div v-if="enabled" class="oauth-button-wrap">
    <button class="oauth-button" type="button" :disabled="loading" @click="signIn">
      {{ loading ? 'Đang chuyển hướng…' : 'Tiếp tục với ' + providerLabel(provider) }}
    </button>
    <p v-if="error" role="alert">{{ error }}</p>
  </div>
</template>
<style scoped>
.oauth-button-wrap {
  width: 100%;
}
.oauth-button {
  width: 100%;
  padding: 0.85rem 1rem;
  border: 1px solid #b9c4c0;
  border-radius: 2rem;
  background: #fff;
  color: #193d33;
  font: inherit;
  cursor: pointer;
}
.oauth-button:disabled {
  opacity: 0.6;
  cursor: wait;
}
.oauth-button:focus-visible {
  outline: 3px solid #edc657;
  outline-offset: 2px;
}
p {
  color: #a4392c;
  font-size: 0.875rem;
  margin: 0.5rem 0;
}
</style>
