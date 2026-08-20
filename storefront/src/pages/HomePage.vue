<script setup lang="ts">
import { computed, onMounted } from 'vue'

import { useBooksStore } from '@/features/books/model/books.store'
import BookCard from '@/features/books/ui/BookCard.vue'
import BookCover from '@/features/books/ui/BookCover.vue'
import AppIcon from '@/shared/ui/AppIcon.vue'

const booksStore = useBooksStore()
const featuredBooks = computed(() => booksStore.books.slice(0, 4))
const heroBook = computed(() => booksStore.books[0])

onMounted(() => booksStore.fetchInitial())
</script>

<template>
  <section class="hero">
    <div class="shell hero__grid">
      <div class="hero__copy">
        <p class="eyebrow"><AppIcon name="sparkles" :size="16" /> Tuyển chọn cho người yêu sách</p>
        <h1>Những trang sách<br /><em>ở lại</em> cùng bạn.</h1>
        <p class="hero__lead">
          Một tủ sách nhỏ được tuyển chọn cẩn thận — dành cho những ngày bạn muốn hiểu thế giới, và
          cả chính mình, sâu hơn một chút.
        </p>
        <div class="hero__actions">
          <RouterLink to="/sach" class="button button--primary">
            Khám phá tủ sách <AppIcon name="arrow-right" :size="18" />
          </RouterLink>
          <a href="#cau-chuyen" class="button button--text">Câu chuyện Mộc Thư</a>
        </div>
        <div class="hero__proof">
          <div><strong>100%</strong><span>Sách tuyển chọn</span></div>
          <div><strong>48h</strong><span>Giao hàng nhanh</span></div>
          <div><strong>4.9/5</strong><span>Từ độc giả</span></div>
        </div>
      </div>

      <div class="hero__visual" aria-hidden="true">
        <span class="hero__sun" />
        <div class="hero__quote">
          “Một cuốn sách hay là một cuộc đối thoại không bao giờ kết thúc.”
        </div>
        <BookCover
          v-if="heroBook"
          :title="heroBook.title"
          :author="heroBook.author"
          :isbn="heroBook.isbn"
          size="large"
        />
        <BookCover
          v-else
          title="Những ngày đọc chậm"
          author="Mộc Thư tuyển chọn"
          isbn="9780000000001"
          size="large"
        />
        <span class="hero__leaf hero__leaf--one" />
        <span class="hero__leaf hero__leaf--two" />
      </div>
    </div>
  </section>

  <section class="book-section section">
    <div class="shell">
      <div class="section-heading">
        <div>
          <p class="eyebrow">Vừa lên kệ</p>
          <h2>Sách mới trong tuần</h2>
        </div>
        <RouterLink to="/sach" class="text-link"
          >Xem tất cả <AppIcon name="arrow-right" :size="17"
        /></RouterLink>
      </div>

      <div v-if="booksStore.loading" class="book-grid" aria-label="Đang tải sách">
        <div v-for="item in 4" :key="item" class="book-skeleton"><span /><i /><i /></div>
      </div>
      <div v-else-if="booksStore.error" class="inline-error">
        <p>{{ booksStore.error }}</p>
        <button class="button button--outline" type="button" @click="booksStore.fetchInitial(true)">
          Thử lại
        </button>
      </div>
      <div v-else-if="featuredBooks.length" class="book-grid">
        <BookCard v-for="book in featuredBooks" :key="book.id" :book="book" />
      </div>
      <div v-else class="empty-inline">Tủ sách đang được chuẩn bị. Mời bạn quay lại sớm nhé.</div>
    </div>
  </section>

  <section id="cau-chuyen" class="story section">
    <div class="shell story__grid">
      <div class="story__art" aria-hidden="true">
        <div class="story__paper"><span>ĐỌC</span><b>chậm</b><span>SỐNG</span><b>sâu</b></div>
      </div>
      <div class="story__copy">
        <p class="eyebrow">Câu chuyện của chúng tôi</p>
        <h2>Không bán nhiều sách.<br />Chỉ giới thiệu sách đáng đọc.</h2>
        <p>
          Giữa một thế giới luôn vội, Mộc Thư tin rằng đọc là một cách để ta trở về. Mỗi tựa sách ở
          đây đều được chọn vì một lý do: nó có điều gì đó chân thật để nói.
        </p>
        <ul>
          <li><AppIcon name="check" :size="18" /> Tuyển chọn bởi những người đọc thực sự</li>
          <li><AppIcon name="check" :size="18" /> Đóng gói tối giản, thân thiện với môi trường</li>
          <li>
            <AppIcon name="check" :size="18" /> Gợi ý sách bằng sự thấu hiểu, không bằng thuật toán
          </li>
        </ul>
      </div>
    </div>
  </section>

  <section class="newsletter">
    <div class="shell newsletter__inner">
      <div>
        <p class="eyebrow">Lá thư mỗi tháng</p>
        <h2>Một cuốn sách, một suy nghĩ,<br />không một email thừa.</h2>
      </div>
      <form class="newsletter__form" @submit.prevent>
        <label class="sr-only" for="newsletter-email">Email</label>
        <input id="newsletter-email" type="email" placeholder="ban@email.com" required />
        <button type="submit" class="button button--accent">Đăng ký</button>
      </form>
    </div>
  </section>
</template>

<style scoped>
.hero {
  overflow: hidden;
  padding: 76px 0 86px;
  background: var(--color-paper);
}
.hero__grid {
  display: grid;
  grid-template-columns: minmax(0, 1.03fr) minmax(380px, 0.97fr);
  gap: 72px;
  align-items: center;
}
.hero__copy h1 {
  max-width: 690px;
  margin: 18px 0 24px;
  font-family: var(--font-display);
  font-size: clamp(3.25rem, 6.5vw, 6.5rem);
  font-weight: 550;
  letter-spacing: -0.055em;
  line-height: 0.9;
}
.hero__copy h1 em {
  color: var(--color-accent-dark);
  font-weight: 500;
}
.hero__lead {
  max-width: 610px;
  margin: 0;
  color: var(--color-muted);
  font-size: 1.05rem;
  line-height: 1.8;
}
.hero__actions {
  display: flex;
  gap: 22px;
  align-items: center;
  margin-top: 32px;
}
.hero__proof {
  display: flex;
  gap: 38px;
  margin-top: 54px;
}
.hero__proof div {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.hero__proof strong {
  color: var(--color-brand);
  font-family: var(--font-display);
  font-size: 1.35rem;
}
.hero__proof span {
  color: var(--color-muted);
  font-size: 0.72rem;
}
.hero__visual {
  position: relative;
  display: grid;
  min-height: 570px;
  place-items: center;
  border-radius: 46% 46% 12px 12px;
  background: #d9cbaa;
}
.hero__visual :deep(.book-cover) {
  z-index: 2;
  transform: rotate(5deg);
}
.hero__sun {
  position: absolute;
  top: 8%;
  right: 8%;
  width: 120px;
  height: 120px;
  border-radius: 50%;
  background: #e8ad57;
  opacity: 0.82;
}
.hero__quote {
  position: absolute;
  z-index: 3;
  bottom: 8%;
  left: -8%;
  width: 210px;
  padding: 18px 20px;
  border-radius: 4px;
  color: var(--color-brand);
  background: #fffaf0;
  box-shadow: var(--shadow-md);
  font-family: var(--font-display);
  font-size: 0.9rem;
  font-style: italic;
  line-height: 1.5;
  transform: rotate(-3deg);
}
.hero__leaf {
  position: absolute;
  width: 52px;
  height: 110px;
  border-radius: 100% 0 100% 0;
  background: var(--color-brand);
  opacity: 0.8;
}
.hero__leaf--one {
  right: -8px;
  bottom: 10%;
  transform: rotate(25deg);
}
.hero__leaf--two {
  top: 22%;
  left: 5%;
  width: 34px;
  height: 76px;
  transform: rotate(-38deg);
  opacity: 0.38;
}
.section-heading {
  display: flex;
  align-items: end;
  justify-content: space-between;
  margin-bottom: 36px;
}
.section-heading h2,
.story h2,
.newsletter h2 {
  margin: 8px 0 0;
  font-family: var(--font-display);
  font-size: clamp(2rem, 4vw, 3.35rem);
  font-weight: 550;
  letter-spacing: -0.035em;
  line-height: 1.04;
}
.text-link {
  display: inline-flex;
  gap: 6px;
  align-items: center;
  padding-bottom: 4px;
  border-bottom: 1px solid var(--color-brand);
  color: var(--color-brand);
  font-size: 0.85rem;
  font-weight: 750;
}
.book-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 26px;
}
.book-skeleton span {
  display: block;
  min-height: 310px;
  border-radius: var(--radius-lg);
  background: linear-gradient(90deg, #e8e2d7 25%, #f2eee6 45%, #e8e2d7 65%);
  background-size: 300% 100%;
  animation: shimmer 1.5s infinite;
}
.book-skeleton i {
  display: block;
  width: 80%;
  height: 14px;
  margin-top: 14px;
  border-radius: 8px;
  background: #e8e2d7;
}
.book-skeleton i:last-child {
  width: 52%;
  margin-top: 8px;
}
.story {
  background: var(--color-surface);
}
.story__grid {
  display: grid;
  grid-template-columns: 0.9fr 1.1fr;
  gap: 90px;
  align-items: center;
}
.story__art {
  display: grid;
  min-height: 490px;
  place-items: center;
  background: #173f35;
  background-image: radial-gradient(circle at 20% 10%, #315c50, transparent 40%);
}
.story__paper {
  display: grid;
  width: 58%;
  min-height: 330px;
  place-content: center;
  padding: 30px;
  color: var(--color-brand);
  background: #f3e6c8;
  box-shadow: 16px 18px 0 #d4a34f;
  text-align: center;
  transform: rotate(-4deg);
}
.story__paper span {
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.25em;
}
.story__paper b {
  margin: 5px 0 18px;
  font-family: var(--font-display);
  font-size: 3rem;
  font-style: italic;
  font-weight: 500;
  line-height: 0.8;
}
.story__copy > p:not(.eyebrow) {
  max-width: 600px;
  margin: 24px 0;
  color: var(--color-muted);
  line-height: 1.8;
}
.story__copy ul {
  display: grid;
  gap: 14px;
  padding: 0;
  list-style: none;
}
.story__copy li {
  display: flex;
  gap: 10px;
  align-items: center;
  font-size: 0.9rem;
}
.story__copy li svg {
  color: var(--color-accent-dark);
}
.newsletter {
  padding: 76px 0;
  color: white;
  background: var(--color-brand);
}
.newsletter__inner {
  display: grid;
  grid-template-columns: 1fr 0.8fr;
  gap: 70px;
  align-items: center;
}
.newsletter .eyebrow {
  color: var(--color-accent);
}
.newsletter__form {
  display: flex;
  gap: 10px;
  padding: 7px;
  border: 1px solid rgb(255 255 255 / 28%);
  border-radius: 12px;
  background: rgb(255 255 255 / 7%);
}
.newsletter__form input {
  min-width: 0;
  flex: 1;
  padding: 10px 12px;
  border: 0;
  outline: 0;
  color: white;
  background: transparent;
  font: inherit;
}
.newsletter__form input::placeholder {
  color: rgb(255 255 255 / 50%);
}
@keyframes shimmer {
  to {
    background-position: -300% 0;
  }
}
@media (max-width: 980px) {
  .hero__grid {
    grid-template-columns: 1fr 380px;
    gap: 30px;
  }
  .book-grid {
    grid-template-columns: repeat(2, 1fr);
    row-gap: 48px;
  }
}
@media (max-width: 760px) {
  .hero {
    padding: 44px 0 60px;
  }
  .hero__grid,
  .story__grid,
  .newsletter__inner {
    grid-template-columns: 1fr;
  }
  .hero__visual {
    min-height: 470px;
    margin-top: 20px;
  }
  .hero__quote {
    left: -2%;
  }
  .story__grid {
    gap: 48px;
  }
  .story__art {
    min-height: 390px;
  }
  .newsletter__form {
    flex-direction: column;
  }
}
@media (max-width: 520px) {
  .hero__copy h1 {
    font-size: 3.35rem;
  }
  .hero__actions {
    align-items: flex-start;
    flex-direction: column;
  }
  .hero__proof {
    gap: 20px;
    justify-content: space-between;
  }
  .hero__proof span {
    font-size: 0.64rem;
  }
  .hero__visual {
    min-height: 420px;
  }
  .hero__quote {
    width: 180px;
  }
  .book-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 16px;
    row-gap: 38px;
  }
  .section-heading {
    align-items: flex-start;
    flex-direction: column;
    gap: 20px;
  }
}
</style>
