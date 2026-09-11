<template>
  <section class="catalog-panel" style="padding: 2rem">
    <h2>Bạn chưa có quyền sử dụng chức năng này</h2>
    <p>Chọn chức năng được cấp quyền trong menu, hoặc liên hệ quản trị viên.</p>
    <RouterLink class="button button--secondary" :to="{ name: 'dashboard' }"
      >Về trang được phép</RouterLink
    >
  </section>
</template>
