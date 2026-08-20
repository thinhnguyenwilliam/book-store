<template>
  <footer class="site-footer">
    <div class="shell site-footer__grid">
      <div>
        <RouterLink to="/" class="footer-brand">Mộc Thư</RouterLink>
        <p>Một hiệu sách nhỏ cho những tâm trí luôn rộng mở.</p>
      </div>
      <div>
        <h2>Khám phá</h2>
        <RouterLink to="/sach">Tất cả sách</RouterLink>
        <RouterLink to="/gio-hang">Giỏ hàng</RouterLink>
      </div>
      <div>
        <h2>Hỗ trợ</h2>
        <a href="mailto:hello@mocthu.vn">hello@mocthu.vn</a>
        <span>Thứ Hai–Thứ Bảy, 8:00–18:00</span>
      </div>
    </div>
    <div class="shell site-footer__bottom">
      <span>© {{ new Date().getFullYear() }} Mộc Thư</span>
      <span>Đọc chậm. Sống sâu.</span>
    </div>
  </footer>
</template>

<style scoped>
.site-footer {
  margin-top: 100px;
  padding: 64px 0 24px;
  color: rgb(255 255 255 / 72%);
  background: #12372f;
}
.site-footer__grid {
  display: grid;
  grid-template-columns: 2fr 1fr 1.4fr;
  gap: 64px;
}
.footer-brand {
  color: white;
  font-family: var(--font-display);
  font-size: 1.8rem;
  font-weight: 750;
}
.site-footer p {
  max-width: 320px;
  margin: 16px 0 0;
  line-height: 1.7;
}
.site-footer h2 {
  margin: 4px 0 18px;
  color: var(--color-accent);
  font-size: 0.7rem;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}
.site-footer__grid > div:not(:first-child) {
  display: flex;
  flex-direction: column;
  gap: 11px;
}
.site-footer a:not(.footer-brand):hover {
  color: white;
}
.site-footer__bottom {
  display: flex;
  justify-content: space-between;
  margin-top: 58px;
  padding-top: 22px;
  border-top: 1px solid rgb(255 255 255 / 12%);
  font-size: 0.74rem;
}
@media (max-width: 760px) {
  .site-footer {
    margin-top: 70px;
  }
  .site-footer__grid {
    grid-template-columns: 1fr;
    gap: 36px;
  }
}
</style>
