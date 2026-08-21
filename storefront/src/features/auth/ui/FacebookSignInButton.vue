<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { initializeFacebook, requestFacebookLogin } from '@/shared/lib/facebook-identity'

const props = defineProps<{
  appId: string
  graphVersion: string
  disabled?: boolean
}>()

const emit = defineEmits<{
  accessToken: [accessToken: string]
  error: [message: string]
}>()

const ready = ref(false)
const effectiveDisabled = computed(() => props.disabled || !ready.value)

onMounted(async () => {
  if (!props.appId) return
  try {
    await initializeFacebook(props.appId, props.graphVersion)
    ready.value = true
  } catch {
    emit('error', 'Không thể tải đăng nhập Facebook. Vui lòng thử lại.')
  }
})

async function signIn(): Promise<void> {
  if (effectiveDisabled.value) return
  try {
    emit('accessToken', await requestFacebookLogin())
  } catch {
    emit('error', 'Bạn đã hủy hoặc chưa cấp quyền đăng nhập Facebook.')
  }
}
</script>

<template>
  <button
    v-if="appId"
    type="button"
    class="facebook-auth-button"
    :disabled="effectiveDisabled"
    @click="signIn"
  >
    <span class="facebook-auth-button__icon" aria-hidden="true">f</span>
    Tiếp tục với Facebook
  </button>
  <p v-else class="google-auth__unavailable">
    Đăng nhập Facebook chưa được cấu hình cho môi trường này.
  </p>
</template>
