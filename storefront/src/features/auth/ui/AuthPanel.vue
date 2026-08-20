<script setup lang="ts">
defineProps<{
  eyebrow: string
  title: string
  subtitle: string
}>()
</script>

<template>
  <section class="auth-page">
    <div class="auth-page__art" aria-hidden="true">
      <div class="auth-page__quote">
        <span>“</span>
        <p>Sách là tấm gương: bạn chỉ nhìn thấy trong đó những gì đã có sẵn bên trong mình.</p>
        <small>— Carlos Ruiz Zafón</small>
      </div>
      <div class="auth-page__rings" />
    </div>
    <div class="auth-page__content">
      <div class="auth-card">
        <RouterLink to="/" class="auth-card__brand">Mộc Thư</RouterLink>
        <p class="eyebrow">{{ eyebrow }}</p>
        <h1>{{ title }}</h1>
        <p class="auth-card__subtitle">{{ subtitle }}</p>
        <slot />
      </div>
    </div>
  </section>
</template>

<style scoped>
.auth-page {
  display: grid;
  min-height: calc(100vh - 76px);
  grid-template-columns: 0.85fr 1.15fr;
}
.auth-page__art {
  position: relative;
  display: grid;
  overflow: hidden;
  min-height: 680px;
  place-items: end start;
  padding: 70px;
  color: white;
  background: var(--color-brand);
  background-image: radial-gradient(circle at 65% 18%, #3c6b5e, transparent 36%);
}
.auth-page__art::before {
  position: absolute;
  top: 11%;
  left: 18%;
  width: 190px;
  height: 280px;
  border: 1px solid rgb(244 201 93 / 48%);
  border-radius: 100px 100px 0 0;
  content: '';
  transform: rotate(12deg);
}
.auth-page__rings {
  position: absolute;
  top: 15%;
  left: 22%;
  width: 250px;
  height: 350px;
  border: 1px solid rgb(255 255 255 / 12%);
  border-radius: 120px 120px 0 0;
  transform: rotate(-10deg);
}
.auth-page__quote {
  position: relative;
  z-index: 1;
  max-width: 430px;
}
.auth-page__quote > span {
  color: var(--color-accent);
  font-family: var(--font-display);
  font-size: 5rem;
  line-height: 0.5;
}
.auth-page__quote p {
  margin: 18px 0;
  font-family: var(--font-display);
  font-size: clamp(1.8rem, 3vw, 2.7rem);
  font-style: italic;
  line-height: 1.15;
}
.auth-page__quote small {
  color: rgb(255 255 255 / 55%);
}
.auth-page__content {
  display: grid;
  place-items: center;
  padding: 60px 28px;
}
.auth-card {
  width: min(100%, 470px);
}
.auth-card__brand {
  display: block;
  margin-bottom: 60px;
  color: var(--color-brand);
  font-family: var(--font-display);
  font-size: 1.45rem;
  font-weight: 750;
}
.auth-card h1 {
  margin: 10px 0;
  font-family: var(--font-display);
  font-size: clamp(2.5rem, 5vw, 4rem);
  font-weight: 550;
  letter-spacing: -0.045em;
}
.auth-card__subtitle {
  margin: 0 0 32px;
  color: var(--color-muted);
  line-height: 1.65;
}
@media (max-width: 820px) {
  .auth-page {
    grid-template-columns: 1fr;
  }
  .auth-page__art {
    display: none;
  }
  .auth-card__brand {
    margin-bottom: 42px;
  }
}
</style>
