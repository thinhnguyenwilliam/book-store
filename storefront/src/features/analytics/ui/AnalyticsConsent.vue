<script setup lang="ts">
import { ref } from 'vue'
import { analyticsConfigured, analyticsConsent, setAnalyticsConsent } from '../lib/google-analytics'

const editing = ref(false)
function choose(value: 'granted' | 'denied'): void {
  setAnalyticsConsent(value)
  editing.value = false
}
</script>

<template>
  <aside v-if="analyticsConfigured" class="analytics-consent" aria-label="Tùy chọn thống kê">
    <section v-if="analyticsConsent === 'unset' || editing" class="analytics-consent__panel">
      <strong>Cho phép thống kê truy cập?</strong>
      <p>
        Mộc Thư dùng Google Analytics để hiểu lượt xem trang, thiết bị và hoạt động xem sách, thêm
        giỏ hàng. Chỉ tải công cụ khi bạn đồng ý; không gửi tên, email hay nội dung chat. Bạn có thể
        thay đổi lựa chọn bất cứ lúc nào.
      </p>
      <div class="analytics-consent__actions">
        <button type="button" @click="choose('denied')">Từ chối</button>
        <button type="button" @click="choose('granted')">Đồng ý</button>
      </div>
    </section>
    <button v-else type="button" @click="editing = true">Tùy chọn thống kê</button>
  </aside>
</template>

<style scoped>
.analytics-consent {
  position: fixed;
  bottom: 1rem;
  left: 1rem;
  z-index: 80;
  max-width: min(26rem, calc(100vw - 2rem));
}
.analytics-consent__panel {
  padding: 1.25rem;
  border: 1px solid #c9c4b7;
  border-radius: 1rem;
  background: #fffdf7;
  color: #1d4036;
  box-shadow: 0 4px 24px #0002;
}
.analytics-consent p {
  margin: 0.75rem 0;
  font-size: 0.875rem;
  line-height: 1.6;
}
.analytics-consent__actions {
  display: flex;
  gap: 0.75rem;
}
.analytics-consent button {
  padding: 0.6rem 0.9rem;
  border: 1px solid #1d4036;
  border-radius: 0.5rem;
  color: #1d4036;
  background: #fffdf7;
  cursor: pointer;
}
.analytics-consent button:focus-visible {
  outline: 3px solid #f1cb59;
  outline-offset: 2px;
}
</style>
