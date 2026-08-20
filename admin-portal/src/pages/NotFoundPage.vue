<script setup lang="ts">
import AppIcon from '@/shared/ui/AppIcon.vue'
</script>

<template>
  <main class="not-found">
    <div>
      <span>404</span>
      <h1>Không tìm thấy trang</h1>
      <p>Đường dẫn này không tồn tại trong khu vực quản trị.</p>
      <RouterLink class="button button--primary" to="/"
        ><AppIcon name="dashboard" :size="17" /> Về tổng quan</RouterLink
      >
    </div>
  </main>
</template>
