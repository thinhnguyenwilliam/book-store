<template>
  <section class="not-found shell">
    <span>404</span>
    <p class="eyebrow">Trang sách bị thất lạc</p>
    <h1>Có vẻ bạn đã đọc<br />lạc sang một trang khác.</h1>
    <RouterLink to="/" class="button button--primary">Về trang chủ</RouterLink>
  </section>
</template>

<style scoped>
.not-found {
  position: relative;
  display: grid;
  min-height: 70vh;
  place-items: center;
  align-content: center;
  overflow: hidden;
  text-align: center;
}
.not-found > span {
  position: absolute;
  z-index: -1;
  color: var(--color-surface);
  font-family: var(--font-display);
  font-size: min(45vw, 26rem);
  font-weight: 800;
  line-height: 1;
}
.not-found h1 {
  margin: 12px 0 30px;
  font-family: var(--font-display);
  font-size: clamp(2.5rem, 6vw, 5rem);
  font-weight: 550;
  letter-spacing: -0.05em;
  line-height: 0.98;
}
</style>
