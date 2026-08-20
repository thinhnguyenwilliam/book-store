<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    title: string
    author: string
    isbn: string
    size?: 'small' | 'medium' | 'large'
  }>(),
  { size: 'medium' },
)

const palette = computed(() => {
  const seed = [...props.isbn].reduce((total, character) => total + character.charCodeAt(0), 0)
  return seed % 5
})
</script>

<template>
  <div class="book-cover" :class="[`book-cover--${size}`, `book-cover--palette-${palette}`]">
    <div class="book-cover__spine" />
    <div class="book-cover__content">
      <span class="book-cover__mark">MỘC THƯ</span>
      <h3>{{ title }}</h3>
      <span class="book-cover__line" />
      <p>{{ author }}</p>
    </div>
  </div>
</template>

<style scoped>
.book-cover {
  --cover-bg: #1d5a4b;
  --cover-ink: #f7edcf;
  position: relative;
  display: grid;
  aspect-ratio: 0.68;
  overflow: hidden;
  border-radius: 4px 11px 11px 4px;
  color: var(--cover-ink);
  background: var(--cover-bg);
  box-shadow:
    -8px 10px 20px rgb(20 43 36 / 12%),
    0 22px 36px rgb(20 43 36 / 16%);
  isolation: isolate;
}

.book-cover::after {
  position: absolute;
  inset: 0;
  z-index: -1;
  background-image:
    radial-gradient(circle at 85% 12%, rgb(255 255 255 / 16%), transparent 30%),
    repeating-linear-gradient(120deg, transparent 0 18px, rgb(255 255 255 / 3%) 18px 19px);
  content: '';
}

.book-cover--small {
  width: 76px;
}
.book-cover--medium {
  width: min(100%, 190px);
}
.book-cover--large {
  width: min(72vw, 310px);
}
.book-cover--palette-1 {
  --cover-bg: #ad4d3f;
  --cover-ink: #fff4df;
}
.book-cover--palette-2 {
  --cover-bg: #355b78;
  --cover-ink: #f2dfae;
}
.book-cover--palette-3 {
  --cover-bg: #6a5039;
  --cover-ink: #fff1d4;
}
.book-cover--palette-4 {
  --cover-bg: #584478;
  --cover-ink: #f5e7c8;
}

.book-cover__spine {
  position: absolute;
  inset: 0 auto 0 0;
  width: 8%;
  border-right: 1px solid rgb(255 255 255 / 16%);
  background: rgb(0 0 0 / 12%);
}

.book-cover__content {
  display: flex;
  flex-direction: column;
  height: 100%;
  padding: 15% 10% 11% 16%;
}

.book-cover__mark {
  font-size: clamp(0.42rem, 2vw, 0.62rem);
  font-weight: 800;
  letter-spacing: 0.18em;
  opacity: 0.72;
}

.book-cover h3 {
  margin: auto 0 0;
  font-family: var(--font-display);
  font-size: clamp(0.86rem, 4vw, 1.45rem);
  line-height: 1.06;
}

.book-cover__line {
  width: 32%;
  height: 1px;
  margin: 12% 0 9%;
  background: currentColor;
  opacity: 0.7;
}

.book-cover p {
  margin: 0;
  font-size: clamp(0.48rem, 2vw, 0.72rem);
  line-height: 1.35;
  opacity: 0.8;
}
</style>
