<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import { useAuthStore } from '@/features/auth/model/auth.store'
import { useNotificationStore } from '@/features/notifications/model/notification.store'
import { ApiError } from '@/shared/api/http-client'
import { formatDate, initials } from '@/shared/lib/format'
import AppIcon from '@/shared/ui/AppIcon.vue'

const auth = useAuthStore()
const notifications = useNotificationStore()
const router = useRouter()
const displayName = ref('')
const saving = ref(false)
const error = ref('')
const memberSince = computed(() => (auth.profile ? formatDate(auth.profile.created_at) : '—'))

watch(
  () => auth.profile?.display_name,
  (value) => {
    displayName.value = value ?? ''
  },
  { immediate: true },
)

async function save(): Promise<void> {
  if (!displayName.value.trim()) return
  saving.value = true
  error.value = ''
  try {
    await auth.saveDisplayName(displayName.value.trim())
    notifications.show('Thông tin tài khoản đã được cập nhật.', 'success')
  } catch (requestError) {
    error.value =
      requestError instanceof ApiError ? requestError.message : 'Không thể cập nhật hồ sơ.'
  } finally {
    saving.value = false
  }
}

async function signOut(): Promise<void> {
  await auth.signOut()
  await router.push('/')
}
</script>

<template>
  <section class="page-hero page-hero--profile">
    <div class="shell profile-hero">
      <span class="profile-avatar">{{ initials(auth.displayName) }}</span>
      <div>
        <p class="eyebrow">Không gian độc giả</p>
        <h1>Chào, {{ auth.displayName }}.</h1>
      </div>
    </div>
  </section>

  <section class="section profile-page">
    <div class="shell profile-grid">
      <aside class="profile-menu">
        <span class="is-active"><AppIcon name="user" :size="18" /> Hồ sơ cá nhân</span>
        <RouterLink to="/gio-hang"><AppIcon name="bag" :size="18" /> Giỏ sách</RouterLink>
        <button type="button" @click="signOut">
          <AppIcon name="logout" :size="18" /> Đăng xuất
        </button>
      </aside>
      <div class="profile-card">
        <div class="profile-card__heading">
          <div>
            <p class="eyebrow">Thông tin cá nhân</p>
            <h2>Hồ sơ của bạn</h2>
          </div>
          <span>Thành viên từ {{ memberSince }}</span>
        </div>
        <form @submit.prevent="save">
          <div v-if="error" class="form-error">{{ error }}</div>
          <label
            ><span>Tên hiển thị</span
            ><input v-model="displayName" type="text" maxlength="120" required
          /></label>
          <label
            ><span>Email</span><input :value="auth.profile?.email" type="email" disabled /><small
              >Email được quản lý bởi Auth Service và hiện chưa hỗ trợ thay đổi.</small
            ></label
          >
          <button class="button button--primary" type="submit" :disabled="saving">
            {{ saving ? 'Đang lưu…' : 'Lưu thay đổi' }}
          </button>
        </form>
      </div>
    </div>
  </section>
</template>

<style scoped>
.page-hero--profile {
  padding: 60px 0;
  color: white;
  background: var(--color-brand);
}
.profile-hero {
  display: flex;
  gap: 24px;
  align-items: center;
}
.profile-avatar {
  display: grid;
  width: 82px;
  height: 82px;
  place-items: center;
  border: 1px solid rgb(255 255 255 / 30%);
  border-radius: 50%;
  color: var(--color-brand);
  background: var(--color-accent);
  font-family: var(--font-display);
  font-size: 1.5rem;
  font-weight: 750;
}
.profile-hero .eyebrow {
  color: var(--color-accent);
}
.profile-hero h1 {
  margin: 6px 0 0;
  font-family: var(--font-display);
  font-size: clamp(2.3rem, 5vw, 4rem);
  font-weight: 550;
  letter-spacing: -0.04em;
}
.profile-grid {
  display: grid;
  grid-template-columns: 240px minmax(0, 1fr);
  gap: 60px;
  align-items: start;
}
.profile-menu {
  display: grid;
  gap: 5px;
}
.profile-menu a,
.profile-menu span,
.profile-menu button {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 13px 14px;
  border: 0;
  border-radius: 10px;
  color: var(--color-muted);
  background: transparent;
  font: inherit;
  font-size: 0.84rem;
  font-weight: 650;
  text-align: left;
}
.profile-menu .is-active {
  color: white;
  background: var(--color-brand);
}
.profile-menu button {
  cursor: pointer;
}
.profile-menu button:hover,
.profile-menu a:hover {
  color: var(--color-brand);
  background: var(--color-surface);
}
.profile-card {
  padding: 38px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-lg);
  background: white;
}
.profile-card__heading {
  display: flex;
  align-items: end;
  justify-content: space-between;
  margin-bottom: 32px;
  padding-bottom: 24px;
  border-bottom: 1px solid var(--color-line);
}
.profile-card__heading h2 {
  margin: 6px 0 0;
  font-family: var(--font-display);
  font-size: 2rem;
}
.profile-card__heading > span {
  color: var(--color-muted);
  font-size: 0.74rem;
}
.profile-card form {
  display: grid;
  gap: 22px;
}
.profile-card label {
  display: grid;
  gap: 8px;
}
.profile-card label > span {
  font-size: 0.76rem;
  font-weight: 750;
}
.profile-card input {
  padding: 14px 15px;
  border: 1px solid var(--color-line);
  border-radius: 10px;
  outline: 0;
  background: var(--color-paper);
  font: inherit;
}
.profile-card input:focus {
  border-color: var(--color-brand);
  box-shadow: 0 0 0 3px rgb(23 63 53 / 9%);
}
.profile-card input:disabled {
  color: var(--color-muted);
  cursor: not-allowed;
}
.profile-card small {
  color: var(--color-muted);
  line-height: 1.5;
}
.profile-card .button {
  justify-self: start;
}
@media (max-width: 760px) {
  .profile-grid {
    grid-template-columns: 1fr;
    gap: 28px;
  }
  .profile-menu {
    grid-template-columns: repeat(3, 1fr);
  }
  .profile-menu a,
  .profile-menu span,
  .profile-menu button {
    justify-content: center;
  }
  .profile-card__heading {
    align-items: flex-start;
    flex-direction: column;
    gap: 12px;
  }
}
@media (max-width: 520px) {
  .profile-card {
    padding: 24px;
  }
  .profile-menu {
    grid-template-columns: 1fr;
  }
}
</style>
