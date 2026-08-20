<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { useAuthStore } from '@/features/auth/model/auth.store'
import { env } from '@/shared/config/env'
import { initials } from '@/shared/lib/format'
import AppIcon from '@/shared/ui/AppIcon.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const sidebarOpen = ref(false)
const pageTitle = computed(() => route.meta.title || 'Quản trị')

async function signOut(): Promise<void> {
  await auth.signOut()
  await router.replace({ name: 'login' })
}
</script>

<template>
  <div class="admin-shell">
    <button
      v-if="sidebarOpen"
      class="sidebar-backdrop"
      type="button"
      aria-label="Đóng menu"
      @click="sidebarOpen = false"
    />
    <aside class="sidebar" :class="{ 'sidebar--open': sidebarOpen }">
      <div class="brand">
        <span class="brand__mark"><AppIcon name="book" :size="22" /></span>
        <span><strong>Book Store</strong><small>Back-office</small></span>
        <button
          class="sidebar__close"
          type="button"
          aria-label="Đóng menu"
          @click="sidebarOpen = false"
        >
          <AppIcon name="close" />
        </button>
      </div>

      <nav class="navigation" aria-label="Điều hướng quản trị">
        <p>Không gian làm việc</p>
        <RouterLink :to="{ name: 'dashboard' }" @click="sidebarOpen = false">
          <AppIcon name="dashboard" /><span>Tổng quan</span>
        </RouterLink>
        <RouterLink :to="{ name: 'books' }" @click="sidebarOpen = false">
          <AppIcon name="book" /><span>Quản lý sách</span>
        </RouterLink>
        <RouterLink :to="{ name: 'customers' }" @click="sidebarOpen = false">
          <AppIcon name="user" /><span>Khách hàng</span>
        </RouterLink>
        <p class="navigation__section">Liên kết</p>
        <a :href="env.storefrontUrl" target="_blank" rel="noopener noreferrer">
          <AppIcon name="external" /><span>Mở storefront</span>
        </a>
      </nav>

      <div class="sidebar__account">
        <span class="avatar">{{ initials(auth.displayName) }}</span>
        <span class="account-copy"
          ><strong>{{ auth.displayName }}</strong
          ><small>{{ auth.profile?.email }}</small></span
        >
        <button type="button" title="Đăng xuất" aria-label="Đăng xuất" @click="signOut">
          <AppIcon name="logout" :size="18" />
        </button>
      </div>
    </aside>

    <div class="workspace">
      <header class="topbar">
        <button class="mobile-menu" type="button" aria-label="Mở menu" @click="sidebarOpen = true">
          <AppIcon name="menu" />
        </button>
        <div>
          <p>Book Store Admin</p>
          <h1>{{ pageTitle }}</h1>
        </div>
        <a
          class="topbar__store"
          :href="env.storefrontUrl"
          target="_blank"
          rel="noopener noreferrer"
        >
          Xem cửa hàng <AppIcon name="external" :size="16" />
        </a>
      </header>
      <main class="workspace__main"><RouterView /></main>
    </div>
  </div>
</template>
