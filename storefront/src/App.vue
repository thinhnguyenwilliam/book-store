<script setup lang="ts">
import NotificationToast from '@/shared/ui/NotificationToast.vue'
import ChatWidget from '@/features/chat/ui/ChatWidget.vue'
import AnalyticsConsent from '@/features/analytics/ui/AnalyticsConsent.vue'
</script>

<template>
  <RouterView />
  <NotificationToast />
  <ChatWidget />
  <AnalyticsConsent />
</template>
