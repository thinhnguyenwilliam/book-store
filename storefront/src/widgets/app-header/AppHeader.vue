<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { useAuthStore } from '@/features/auth/model/auth.store'
import { useCartStore } from '@/features/cart/model/cart.store'
import { initials } from '@/shared/lib/format'
import AppIcon from '@/shared/ui/AppIcon.vue'

const auth = useAuthStore()
const cart = useCartStore()
const route = useRoute()
const router = useRouter()
const menuOpen = ref(false)

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
