import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

import AdminLayout from '@/app/layouts/AdminLayout.vue'
import { useAuthStore } from '@/features/auth/model/auth.store'

declare module 'vue-router' {
  interface RouteMeta {
    requiresAdmin?: boolean
    guestOnly?: boolean
    title?: string
  }
}

const routes: RouteRecordRaw[] = [
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
        path: '',
        name: 'dashboard',
        component: () => import('@/pages/DashboardPage.vue'),
        meta: { title: 'Tổng quan' },
      },
      {
        path: 'sach',
        name: 'books',
        component: () => import('@/pages/BooksPage.vue'),
        meta: { title: 'Quản lý sách' },
      },
      {
        path: 'khach-hang',
        name: 'customers',
        component: () => import('@/pages/CustomersPage.vue'),
        meta: { title: 'Quản lý khách hàng' },
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
  const auth = useAuthStore()
  await auth.initialize()

  if (to.meta.requiresAdmin && !auth.isAdmin) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.meta.guestOnly && auth.isAdmin) return { name: 'dashboard' }
})

router.afterEach((to) => {
  document.title = `${to.meta.title || 'Quản trị'} · Book Store`
})

export default router
