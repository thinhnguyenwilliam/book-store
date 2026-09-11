import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

import AdminLayout from '@/app/layouts/AdminLayout.vue'
import { useAuthStore } from '@/features/auth/model/auth.store'
import { adminLandingRoute } from '@/features/authorization/landing'

declare module 'vue-router' {
  interface RouteMeta {
    requiresAdmin?: boolean
    guestOnly?: boolean
    title?: string
    permissions?: string[]
  }
}

const routes: RouteRecordRaw[] = [
  {
    path: '/auth/callback/:provider',
    name: 'oauth-callback',
    component: () => import('@/pages/OAuthCallbackPage.vue'),
  },
  {
    path: '/dang-nhap',
    name: 'login',
    component: () => import('@/pages/LoginPage.vue'),
    meta: { guestOnly: true, title: 'Đăng nhập' },
  },
  {
    path: '/',
    component: AdminLayout,
    meta: { requiresAdmin: true },
    children: [
      {
        path: 'phan-quyen',
        name: 'authorization',
        component: () => import('@/pages/AuthorizationPage.vue'),
        meta: { title: 'Vai trò & phân quyền', permissions: ['roles.read'] },
      },
      {
        path: 'khong-co-quyen',
        name: 'forbidden',
        component: () => import('@/pages/ForbiddenPage.vue'),
        meta: { title: 'Không có quyền truy cập' },
      },
      {
        path: '',
        name: 'dashboard',
        component: () => import('@/pages/DashboardPage.vue'),
        meta: {
          title: 'Tổng quan',
          permissions: ['analytics.read', 'books.read', 'customers.read'],
        },
      },
      {
        path: 'sach',
        name: 'books',
        component: () => import('@/pages/BooksPage.vue'),
        meta: { title: 'Quản lý sách', permissions: ['books.read'] },
      },
      {
        path: 'khach-hang',
        name: 'customers',
        component: () => import('@/pages/CustomersPage.vue'),
        meta: { title: 'Quản lý khách hàng', permissions: ['customers.read'] },
      },
      {
        path: 'tro-chuyen',
        name: 'chat',
        component: () => import('@/pages/ChatPage.vue'),
        meta: { title: 'Trò chuyện hỗ trợ', permissions: ['chat.read'] },
      },
    ],
  },
  {
    path: '/:pathMatch(.*)*',
    name: 'not-found',
    component: () => import('@/pages/NotFoundPage.vue'),
    meta: { title: 'Không tìm thấy' },
  },
]

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
  scrollBehavior: () => ({ top: 0 }),
})

router.beforeEach(async (to) => {
  if (to.name === 'oauth-callback') return
  const auth = useAuthStore()
  await auth.initialize()
  if (auth.isAuthenticated) {
    try {
      await auth.refreshPermissions()
    } catch {
      return to.name === 'login' ? true : { name: 'login' }
    }
  }

  if (to.meta.requiresAdmin && !auth.isAdmin) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.meta.guestOnly && auth.isAdmin) return { name: adminLandingRoute(auth.can) }
  if (to.name === 'dashboard') {
    const landing = adminLandingRoute(auth.can)
    if (landing !== 'dashboard') return { name: landing }
  }
  if (to.meta.permissions?.some((permission) => !auth.can(permission))) return { name: 'forbidden' }
})

router.afterEach((to) => {
  document.title = `${to.meta.title || 'Quản trị'} · Book Store`
})

export default router
