import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

import StoreLayout from '@/app/layouts/StoreLayout.vue'
import { useAuthStore } from '@/features/auth/model/auth.store'

declare module 'vue-router' {
  interface RouteMeta {
    requiresAuth?: boolean
    guestOnly?: boolean
  }
}

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    component: StoreLayout,
    children: [
      { path: '', name: 'home', component: () => import('@/pages/HomePage.vue') },
      { path: 'sach', name: 'catalog', component: () => import('@/pages/CatalogPage.vue') },
      {
        path: 'sach/:id',
        name: 'book-detail',
        component: () => import('@/pages/BookDetailPage.vue'),
      },
      { path: 'gio-hang', name: 'cart', component: () => import('@/pages/CartPage.vue') },
      {
        path: 'thanh-toan/ket-qua',
        name: 'payment-result',
        component: () => import('@/pages/PaymentResultPage.vue'),
        meta: { requiresAuth: true },
      },
      {
        path: 'don-hang/:id',
        name: 'order-detail',
        component: () => import('@/pages/OrderDetailPage.vue'),
        meta: { requiresAuth: true },
      },
      {
        path: 'dang-nhap',
        name: 'login',
        component: () => import('@/pages/LoginPage.vue'),
        meta: { guestOnly: true },
      },
      {
        path: 'dang-ky',
        name: 'register',
        component: () => import('@/pages/RegisterPage.vue'),
        meta: { guestOnly: true },
      },
      {
        path: 'tai-khoan',
        name: 'profile',
        component: () => import('@/pages/ProfilePage.vue'),
        meta: { requiresAuth: true },
      },
      {
        path: ':pathMatch(.*)*',
        name: 'not-found',
        component: () => import('@/pages/NotFoundPage.vue'),
      },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
  scrollBehavior(to, _from, savedPosition) {
    if (savedPosition) return savedPosition
    if (to.hash) return { el: to.hash, behavior: 'smooth' }
    return { top: 0 }
  },
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  await auth.initialize()

  if (to.meta.requiresAuth && !auth.isAuthenticated) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.meta.guestOnly && auth.isAuthenticated) {
    return { name: 'profile' }
  }
})

export default router
