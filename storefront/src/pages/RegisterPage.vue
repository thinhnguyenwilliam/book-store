<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import { useAuthStore } from '@/features/auth/model/auth.store'
import AuthPanel from '@/features/auth/ui/AuthPanel.vue'
import FacebookSignInButton from '@/features/auth/ui/FacebookSignInButton.vue'
import GoogleSignInButton from '@/features/auth/ui/GoogleSignInButton.vue'
import { ApiError } from '@/shared/api/http-client'
import { env } from '@/shared/config/env'
import AppIcon from '@/shared/ui/AppIcon.vue'

const auth = useAuthStore()
const router = useRouter()
const form = reactive({ display_name: '', email: '', password: '' })
const error = ref('')

async function submit(): Promise<void> {
  error.value = ''
  try {
    await auth.signUp(form)
    await router.push('/tai-khoan')
  } catch (requestError) {
    error.value =
      requestError instanceof ApiError ? requestError.message : 'Đăng ký không thành công.'
  }
}

async function signUpWithGoogle(credential: string): Promise<void> {
  error.value = ''
  try {
    await auth.signInWithGoogle(credential)
    await router.push('/tai-khoan')
  } catch (requestError) {
    error.value =
      requestError instanceof ApiError ? requestError.message : 'Đăng ký Google không thành công.'
  }
}

async function signUpWithFacebook(accessToken: string): Promise<void> {
  error.value = ''
  try {
    await auth.signInWithFacebook(accessToken)
    await router.push('/tai-khoan')
  } catch (requestError) {
    error.value =
      requestError instanceof ApiError ? requestError.message : 'Đăng ký Facebook không thành công.'
  }
}
</script>

<template>
  <AuthPanel
    eyebrow="Trở thành độc giả"
    title="Tạo tài khoản"
    subtitle="Lưu lại những cuốn sách bạn yêu và bắt đầu một tủ sách riêng."
  >
    <form class="auth-form" @submit.prevent="submit">
      <div v-if="error" class="form-error" role="alert">{{ error }}</div>
      <label>
        <span>Tên hiển thị</span>
        <span class="input-wrap"
          ><AppIcon name="user" :size="18" /><input
            v-model.trim="form.display_name"
            type="text"
            autocomplete="name"
            placeholder="Nguyễn An"
            required
        /></span>
      </label>
      <label>
        <span>Email</span>
        <span class="input-wrap"
          ><AppIcon name="mail" :size="18" /><input
            v-model.trim="form.email"
            type="email"
            autocomplete="email"
            placeholder="ban@email.com"
            required
        /></span>
      </label>
      <label>
        <span>Mật khẩu</span>
        <span class="input-wrap"
          ><AppIcon name="lock" :size="18" /><input
            v-model="form.password"
            type="password"
            autocomplete="new-password"
            minlength="8"
            placeholder="Ít nhất 8 ký tự"
            required
        /></span>
      </label>
      <button class="button button--primary auth-submit" type="submit" :disabled="auth.loading">
        {{ auth.loading ? 'Đang tạo tài khoản…' : 'Tạo tài khoản' }}
        <AppIcon v-if="!auth.loading" name="arrow-right" :size="18" />
      </button>
      <p class="auth-switch">Đã có tài khoản? <RouterLink to="/dang-nhap">Đăng nhập</RouterLink></p>
    </form>
    <div class="auth-divider"><span>hoặc</span></div>
    <div class="auth-social-buttons">
      <GoogleSignInButton
        :client-id="env.googleClientId"
        :disabled="auth.loading"
        text="signup_with"
        @credential="signUpWithGoogle"
        @error="error = $event"
      />
      <FacebookSignInButton
        :app-id="env.facebookAppId"
        :graph-version="env.facebookGraphVersion"
        :disabled="auth.loading"
        @access-token="signUpWithFacebook"
        @error="error = $event"
      />
    </div>
  </AuthPanel>
</template>
