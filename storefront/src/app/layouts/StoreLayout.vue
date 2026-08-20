<script setup lang="ts">
import AppFooter from '@/widgets/app-footer/AppFooter.vue'
import AppHeader from '@/widgets/app-header/AppHeader.vue'
</script>

<template>
  <div class="app-frame">
    <AppHeader />
    <main><RouterView /></main>
    <AppFooter />
  </div>
</template>
