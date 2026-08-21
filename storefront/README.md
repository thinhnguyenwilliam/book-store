# Mộc Thư Storefront

Storefront Vue 3 + TypeScript dành cho Book Store. Ứng dụng dùng Axios gọi Echo API Gateway, hỗ trợ đăng ký/đăng nhập, hồ sơ, danh sách sách bằng cursor pagination, chi tiết sách và giỏ hàng lưu cục bộ.

## Chạy local

Yêu cầu Node.js 22+ và pnpm 9+; backend phải chạy tại `http://localhost:8080`.

```bash
cd storefront
cp .env.example .env
pnpm install
pnpm dev
```

Mở <http://localhost:5173>. Vite tự hot reload khi thay đổi source.

## Cấu hình

```dotenv
VITE_API_BASE_URL=http://localhost:8080
VITE_API_TIMEOUT_MS=10000
VITE_GOOGLE_CLIENT_ID=your-web-client-id.apps.googleusercontent.com
```

`.env` dùng cho máy local và đã được Git bỏ qua. Chỉ commit `.env.example`; mọi biến bắt đầu bằng `VITE_` đều được đưa vào bundle phía trình duyệt nên không đặt password, JWT secret hay khóa riêng trong đó. Google Web Client ID là public identifier nên có thể dùng ở đây; Google client secret tuyệt đối không được đưa vào frontend.

## Kiểm tra và build

```bash
pnpm check
pnpm build
pnpm preview
```

Hoặc dùng `make dev`, `make check`, `make docker-build`.

Build Docker production:

```bash
docker build \
  --build-arg VITE_API_BASE_URL=https://api.example.com \
  -t bookstore-storefront .
docker run --rm -p 4173:80 bookstore-storefront
```

`VITE_API_BASE_URL` được nhúng lúc build. Nginx phục vụ SPA fallback, cache asset fingerprint và endpoint health check `/healthz`.

Khi deploy khác domain, thêm domain storefront vào `gateway.allowed_origins`, bật `gateway.refresh_cookie_secure: true` và dùng HTTPS. Backend local đã cho phép origin `http://localhost:5173`.

## Cấu trúc

```text
src/
├── app/          router, layout và app bootstrap
├── assets/       style toàn cục
├── features/     auth, books, cart, notifications
├── pages/        các route-level view, lazy loaded
├── shared/       API client, config, helper, UI cơ bản
└── widgets/      header và footer
```

Giỏ hàng hiện lưu bằng `localStorage`; checkout chưa gọi API vì backend chưa có Order Service.

Access token chỉ nằm trong memory của tab, không ghi vào `localStorage` hoặc `sessionStorage`. Refresh token nằm trong cookie `HttpOnly` nên JavaScript không thể đọc; Axios bật `withCredentials` và response interceptor chỉ thực hiện một refresh request đồng thời khi access token hết hạn. Backend rotate refresh token sau mỗi lần dùng và revoke khi logout.
