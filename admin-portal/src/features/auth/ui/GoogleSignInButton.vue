<script setup lang="ts">
import { onMounted, ref } from 'vue'

import { renderGoogleButton, type GoogleButtonText } from '@/shared/lib/google-identity'

const props = withDefaults(
  defineProps<{
    clientId: string
    disabled?: boolean
    text?: GoogleButtonText
  }>(),
  {
    disabled: false,
    text: 'signin_with',
  },
)

const emit = defineEmits<{
  credential: [credential: string]
  error: [message: string]
}>()

const buttonHost = ref<HTMLElement>()

onMounted(async () => {
  if (!props.clientId || !buttonHost.value) return
  try {
    await renderGoogleButton(buttonHost.value, props.clientId, props.text, (credential) => {
      if (!props.disabled) emit('credential', credential)
    })
  } catch {
    emit('error', 'Không thể tải đăng nhập Google. Vui lòng thử lại.')
  }
})
</script>

<template>
  <div class="google-auth">
    <div
      v-if="clientId"
      ref="buttonHost"
      class="google-auth__button"
      :class="{ 'google-auth__button--disabled': disabled }"
      :aria-busy="disabled"
    />
    <p v-else class="google-auth__unavailable">
      Đăng nhập Google chưa được cấu hình cho môi trường này.
    </p>
  </div>
</template>
