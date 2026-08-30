<script setup lang="ts">
import NotificationToast from '@/shared/ui/NotificationToast.vue'
import ChatWidget from '@/features/chat/ui/ChatWidget.vue'
</script>

<template>
  <RouterView />
  <NotificationToast />
  <ChatWidget />
</template>
