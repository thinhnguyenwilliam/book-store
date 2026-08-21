<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { useAuthStore } from '@/features/auth/model/auth.store'
import GoogleSignInButton from '@/features/auth/ui/GoogleSignInButton.vue'
import { ApiError } from '@/shared/api/http-client'
import { env } from '@/shared/config/env'
import AppIcon from '@/shared/ui/AppIcon.vue'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const form = reactive({ email: '', password: '' })
const errorMessage = ref('')

async function submit(): Promise<void> {
  errorMessage.value = ''
  try {
    await auth.signIn(form)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/'
    await router.replace(redirect)
  } catch (error) {
    errorMessage.value = error instanceof ApiError ? error.message : 'Không thể đăng nhập.'
  }
}

async function signInWithGoogle(credential: string): Promise<void> {
  errorMessage.value = ''
  try {
    await auth.signInWithGoogle(credential)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/'
    await router.replace(redirect)
  } catch (error) {
    errorMessage.value = error instanceof ApiError ? error.message : 'Không thể đăng nhập Google.'
  }
}
</script>

<template>
  <main class="login-page">
    <section class="login-story">
      <a :href="env.storefrontUrl" target="_blank" rel="noopener" class="login-brand">
        <span><AppIcon name="book" :size="24" /></span>
        <strong>Book Store</strong>
      </a>
      <div class="login-story__copy">
        <p class="eyebrow eyebrow--light">Back-office</p>
        <h1>Điều hành cửa hàng từ một nơi duy nhất.</h1>
        <p>Theo dõi danh mục, kiểm soát tồn kho và cập nhật storefront theo thời gian thực.</p>
      </div>
      <div class="login-story__status">
        <i /><span>Hệ thống quản trị bảo mật bằng access token ngắn hạn</span>
      </div>
    </section>

    <section class="login-panel">
      <div class="login-card">
        <header>
          <p class="eyebrow">Khu vực nội bộ</p>
          <h2>Đăng nhập quản trị</h2>
          <p>Chỉ tài khoản có role <code>admin</code> mới được truy cập.</p>
        </header>

        <div class="login-google">
          <GoogleSignInButton
            :client-id="env.googleClientId"
            :disabled="auth.loading"
            @credential="signInWithGoogle"
            @error="errorMessage = $event"
          />
        </div>
        <div class="login-divider"><span>hoặc đăng nhập bằng mật khẩu</span></div>

        <form class="login-form" @submit.prevent="submit">
          <label
            ><span>Email</span
            ><input
              v-model="form.email"
              type="email"
              autocomplete="username"
              required
              placeholder="admin@bookstore.com"
          /></label>
          <label
            ><span>Mật khẩu</span
            ><input
              v-model="form.password"
              type="password"
              autocomplete="current-password"
              required
              minlength="8"
              placeholder="••••••••"
          /></label>
          <p v-if="errorMessage || auth.accessDenied" class="form-error">
            {{ errorMessage || 'Phiên hiện tại không có quyền quản trị.' }}
          </p>
          <button
            class="button button--primary login-submit"
            type="submit"
            :disabled="auth.loading"
          >
            <span v-if="auth.loading" class="spinner" />
            <template v-else>Đăng nhập <AppIcon name="arrow-right" :size="17" /></template>
          </button>
        </form>
        <a :href="env.storefrontUrl" class="back-to-store" target="_blank" rel="noopener"
          >Quay lại storefront <AppIcon name="external" :size="15"
        /></a>
      </div>
    </section>
  </main>
</template>
