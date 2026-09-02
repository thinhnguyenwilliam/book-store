<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { useAuthStore } from '@/features/auth/model/auth.store'
import { useCartStore } from '@/features/cart/model/cart.store'
import { useInboxStore } from '@/features/notifications/model/inbox.store'
import PushNotificationToggle from '@/features/push/ui/PushNotificationToggle.vue'
import { initials } from '@/shared/lib/format'
import AppIcon from '@/shared/ui/AppIcon.vue'

const auth = useAuthStore()
const cart = useCartStore()
const inbox = useInboxStore()
const route = useRoute()
const router = useRouter()
const menuOpen = ref(false)
const notificationsOpen = ref(false)
let refreshTimer: number | undefined

watch(
  () => auth.isAuthenticated,
  async (authenticated) => {
    window.clearInterval(refreshTimer)
    if (!authenticated) {
      inbox.clear()
      cart.switchToGuest()
      notificationsOpen.value = false
      return
    }
    await cart.syncAuthenticated().catch(() => undefined)
    await inbox.refresh().catch(() => undefined)
    refreshTimer = window.setInterval(() => void inbox.refresh().catch(() => undefined), 30_000)
  },
  { immediate: true },
)

onBeforeUnmount(() => window.clearInterval(refreshTimer))

watch(
  () => route.fullPath,
  () => {
    menuOpen.value = false
  },
)

async function signOut(): Promise<void> {
  await auth.signOut()
  await router.push('/')
}

function notificationTime(value: string): string {
  return new Intl.DateTimeFormat('vi-VN', { dateStyle: 'short', timeStyle: 'short' }).format(
    new Date(value),
  )
}
</script>

<template>
  <header class="site-header">
    <div class="shell site-header__inner">
      <RouterLink to="/" class="brand" aria-label="Mộc Thư — Trang chủ">
        <span class="brand__mark"><AppIcon name="book" :size="20" /></span>
        <span>Mộc Thư</span>
      </RouterLink>

      <nav class="desktop-nav" aria-label="Điều hướng chính">
        <RouterLink to="/">Trang chủ</RouterLink>
        <RouterLink to="/sach">Tủ sách</RouterLink>
        <a href="/#cau-chuyen">Câu chuyện</a>
      </nav>

      <div class="header-actions">
        <div v-if="auth.isAuthenticated" class="notification-menu">
          <button
            class="notification-button"
            type="button"
            :aria-expanded="notificationsOpen"
            aria-label="Thông báo"
            @click="notificationsOpen = !notificationsOpen"
          >
            <AppIcon name="bell" />
            <span v-if="inbox.unreadCount" class="notification-button__badge">{{
              inbox.unreadCount > 99 ? '99+' : inbox.unreadCount
            }}</span>
          </button>
          <div v-if="notificationsOpen" class="notification-panel">
            <div class="notification-panel__head">
              <strong>Thông báo</strong>
              <button type="button" :disabled="!inbox.unreadCount" @click="inbox.readAll">
                Đọc tất cả
              </button>
            </div>
            <PushNotificationToggle />
            <p v-if="inbox.loading && !inbox.items.length" class="notification-empty">Đang tải…</p>
            <p v-else-if="!inbox.items.length" class="notification-empty">
              Bạn chưa có thông báo nào.
            </p>
            <button
              v-for="item in inbox.items"
              v-else
              :key="item.id"
              type="button"
              class="notification-item"
              :class="{ 'notification-item--unread': !item.read_at }"
              @click="inbox.read(item)"
            >
              <span class="notification-item__dot" />
              <span>
                <strong>{{ item.title }}</strong>
                <small>{{ item.body }}</small>
                <time :datetime="item.created_at">{{ notificationTime(item.created_at) }}</time>
              </span>
            </button>
          </div>
        </div>
        <RouterLink
          v-if="auth.isAuthenticated"
          to="/tai-khoan"
          class="account-link"
          aria-label="Tài khoản"
        >
          <span class="avatar">{{ initials(auth.displayName) }}</span>
          <span class="account-link__label">{{ auth.displayName }}</span>
        </RouterLink>
        <RouterLink v-else to="/dang-nhap" class="account-link" aria-label="Đăng nhập">
          <AppIcon name="user" />
          <span class="account-link__label">Đăng nhập</span>
        </RouterLink>
        <RouterLink to="/gio-hang" class="cart-link" aria-label="Giỏ hàng">
          <AppIcon name="bag" />
          <span v-if="cart.itemCount" class="cart-link__badge">{{ cart.itemCount }}</span>
        </RouterLink>
        <button
          class="menu-button"
          type="button"
          :aria-expanded="menuOpen"
          aria-label="Mở menu"
          @click="menuOpen = !menuOpen"
        >
          <AppIcon :name="menuOpen ? 'close' : 'menu'" />
        </button>
      </div>
    </div>

    <Transition name="mobile-nav">
      <nav v-if="menuOpen" class="mobile-nav" aria-label="Điều hướng di động">
        <RouterLink to="/">Trang chủ</RouterLink>
        <RouterLink to="/sach">Tủ sách</RouterLink>
        <a href="/#cau-chuyen">Câu chuyện</a>
        <RouterLink v-if="!auth.isAuthenticated" to="/dang-ky">Tạo tài khoản</RouterLink>
        <button v-else type="button" @click="signOut">
          <AppIcon name="logout" :size="18" /> Đăng xuất
        </button>
      </nav>
    </Transition>
  </header>
</template>

<style scoped>
.site-header {
  position: sticky;
  z-index: 40;
  top: 0;
  border-bottom: 1px solid rgb(27 61 52 / 9%);
  background: rgb(249 247 241 / 90%);
  backdrop-filter: blur(18px);
}
.site-header__inner {
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  min-height: 76px;
}
.brand {
  display: inline-flex;
  gap: 10px;
  align-items: center;
  justify-self: start;
  color: var(--color-brand);
  font-family: var(--font-display);
  font-size: 1.28rem;
  font-weight: 750;
}
.brand__mark {
  display: grid;
  width: 35px;
  height: 35px;
  place-items: center;
  border-radius: 50%;
  color: var(--color-cream);
  background: var(--color-brand);
}
.desktop-nav {
  display: flex;
  gap: 34px;
  align-items: center;
}
.desktop-nav a {
  position: relative;
  padding: 27px 0 24px;
  color: var(--color-muted);
  font-size: 0.88rem;
  font-weight: 650;
}
.desktop-nav a::after {
  position: absolute;
  right: 0;
  bottom: 18px;
  left: 0;
  height: 2px;
  background: var(--color-accent);
  content: '';
  transform: scaleX(0);
  transition: transform 160ms ease;
}
.desktop-nav a:hover,
.desktop-nav a.router-link-active {
  color: var(--color-brand);
}
.desktop-nav a:hover::after,
.desktop-nav a.router-link-active::after {
  transform: scaleX(1);
}
.header-actions {
  display: flex;
  gap: 20px;
  align-items: center;
  justify-self: end;
}
.notification-menu {
  position: relative;
}
.notification-button {
  position: relative;
  display: grid;
  width: 38px;
  height: 38px;
  place-items: center;
  border: 0;
  color: var(--color-ink);
  background: transparent;
  cursor: pointer;
}
.notification-button__badge {
  position: absolute;
  top: -3px;
  right: -5px;
  min-width: 18px;
  padding: 2px 4px;
  border: 2px solid var(--color-paper);
  border-radius: 10px;
  color: white;
  background: #b94735;
  font-size: 0.58rem;
  font-weight: 800;
}
.notification-panel {
  position: absolute;
  top: calc(100% + 14px);
  right: -70px;
  width: min(380px, calc(100vw - 32px));
  overflow: hidden;
  border: 1px solid var(--color-line);
  border-radius: 16px;
  background: var(--color-paper);
  box-shadow: 0 18px 45px rgb(20 48 40 / 16%);
}
.notification-panel__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 18px;
  border-bottom: 1px solid var(--color-line);
}
.notification-panel__head button {
  border: 0;
  color: var(--color-brand);
  background: transparent;
  font-size: 0.75rem;
  font-weight: 700;
  cursor: pointer;
}
.notification-panel__head button:disabled {
  opacity: 0.4;
  cursor: default;
}
.notification-empty {
  margin: 0;
  padding: 28px 18px;
  color: var(--color-muted);
  text-align: center;
}
.notification-item {
  display: grid;
  grid-template-columns: 8px 1fr;
  gap: 10px;
  width: 100%;
  padding: 14px 18px;
  border: 0;
  border-bottom: 1px solid var(--color-line);
  color: var(--color-ink);
  background: transparent;
  text-align: left;
  cursor: pointer;
}
.notification-item:hover,
.notification-item--unread {
  background: #f1f4eb;
}
.notification-item__dot {
  width: 7px;
  height: 7px;
  margin-top: 6px;
  border-radius: 50%;
}
.notification-item--unread .notification-item__dot {
  background: var(--color-accent-dark);
}
.notification-item strong,
.notification-item small,
.notification-item time {
  display: block;
}
.notification-item strong {
  margin-bottom: 4px;
  font-size: 0.84rem;
}
.notification-item small {
  color: var(--color-muted);
  line-height: 1.4;
}
.notification-item time {
  margin-top: 6px;
  color: var(--color-muted);
  font-size: 0.67rem;
}
.account-link {
  display: inline-flex;
  gap: 9px;
  align-items: center;
  color: var(--color-ink);
  font-size: 0.82rem;
  font-weight: 700;
}
.avatar {
  display: grid;
  width: 32px;
  height: 32px;
  place-items: center;
  border-radius: 50%;
  color: white;
  background: var(--color-brand);
  font-size: 0.7rem;
  letter-spacing: 0.04em;
}
.cart-link {
  position: relative;
  display: grid;
  width: 38px;
  height: 38px;
  place-items: center;
  color: var(--color-ink);
}
.cart-link__badge {
  position: absolute;
  top: -2px;
  right: -2px;
  display: grid;
  min-width: 18px;
  height: 18px;
  place-items: center;
  padding: 0 4px;
  border: 2px solid var(--color-paper);
  border-radius: 10px;
  color: white;
  background: var(--color-accent-dark);
  font-size: 0.62rem;
  font-weight: 800;
}
.menu-button {
  display: none;
  padding: 4px;
  border: 0;
  color: var(--color-ink);
  background: transparent;
}
.mobile-nav {
  display: none;
}
@media (max-width: 820px) {
  .site-header__inner {
    grid-template-columns: 1fr auto;
    min-height: 68px;
  }
  .desktop-nav,
  .account-link__label {
    display: none;
  }
  .menu-button {
    display: grid;
    place-items: center;
    cursor: pointer;
  }
  .mobile-nav {
    display: grid;
    gap: 4px;
    padding: 8px 20px 20px;
    border-top: 1px solid var(--color-line);
    background: var(--color-paper);
  }
  .mobile-nav a,
  .mobile-nav button {
    display: flex;
    gap: 8px;
    align-items: center;
    padding: 13px 10px;
    border: 0;
    border-radius: 10px;
    color: var(--color-ink);
    background: transparent;
    font: inherit;
    font-weight: 650;
    text-align: left;
  }
  .mobile-nav a:hover,
  .mobile-nav button:hover {
    background: var(--color-surface);
  }
}
.mobile-nav-enter-active,
.mobile-nav-leave-active {
  transition: 180ms ease;
}
.mobile-nav-enter-from,
.mobile-nav-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
</style>
