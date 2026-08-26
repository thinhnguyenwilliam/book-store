<script setup lang="ts">
import { onMounted, ref } from 'vue'

import * as authApi from '@/features/auth/api/auth.api'
import { renderGoogleButton, type GoogleButtonText } from '@/shared/lib/google-identity'

const props = withDefaults(
  defineProps<{
    clientId: string
    createAccount: boolean
    disabled?: boolean
    text?: GoogleButtonText
  }>(),
  {
    disabled: false,
    text: 'signin_with',
  },
)

const emit = defineEmits<{
  credential: [credential: string, state: string]
  error: [message: string]
}>()

const buttonHost = ref<HTMLElement>()

onMounted(async () => {
  if (!props.clientId || !buttonHost.value) return
  try {
    const transaction = await authApi.createProviderState({
      provider: 'google',
      create_account: props.createAccount,
    })
    await renderGoogleButton(
      buttonHost.value,
      props.clientId,
      props.text,
      transaction.state,
      (credential, returnedState) => {
        if (!props.disabled && returnedState === transaction.state) {
          emit('credential', credential, transaction.state)
        } else if (returnedState !== transaction.state) {
          emit('error', 'Phiên đăng nhập Google không hợp lệ. Vui lòng tải lại trang.')
        }
      },
    )
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
