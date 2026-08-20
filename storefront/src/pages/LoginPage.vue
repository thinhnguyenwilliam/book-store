<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { useAuthStore } from '@/features/auth/model/auth.store'
import AuthPanel from '@/features/auth/ui/AuthPanel.vue'
import { ApiError } from '@/shared/api/http-client'
import AppIcon from '@/shared/ui/AppIcon.vue'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const form = reactive({ email: '', password: '' })
const error = ref('')

async function submit(): Promise<void> {
  error.value = ''
  try {
    await auth.signIn(form)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/tai-khoan'
    await router.push(redirect)
  } catch (requestError) {
    error.value =
      requestError instanceof ApiError ? requestError.message : 'Đăng nhập không thành công.'
  }
}
</script>

<template>
  <AuthPanel
    eyebrow="Chào bạn trở lại"
    title="Đăng nhập"
    subtitle="Tiếp tục hành trình đọc của riêng bạn."
  >
    <form class="auth-form" @submit.prevent="submit">
      <div v-if="error" class="form-error" role="alert">{{ error }}</div>
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
            autocomplete="current-password"
            minlength="8"
            placeholder="Ít nhất 8 ký tự"
            required
        /></span>
      </label>
      <button class="button button--primary auth-submit" type="submit" :disabled="auth.loading">
        {{ auth.loading ? 'Đang đăng nhập…' : 'Đăng nhập' }}
        <AppIcon v-if="!auth.loading" name="arrow-right" :size="18" />
      </button>
      <p class="auth-switch">
        Chưa có tài khoản? <RouterLink to="/dang-ky">Đăng ký ngay</RouterLink>
      </p>
    </form>
  </AuthPanel>
</template>
