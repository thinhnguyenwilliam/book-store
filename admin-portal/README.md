# Book Store Admin Portal

Back-office quản trị Book Store, dùng cùng stack với storefront:

- Vue 3 + TypeScript + Vite
- Pinia
- Vue Router
- Axios
- pnpm

## Chức năng hiện có

- Đăng nhập bằng access token ngắn hạn và refresh token trong cookie HttpOnly.
- Route guard kiểm tra role `admin`; backend vẫn là nơi thực thi authorization cuối cùng.
- Dashboard thống kê dữ liệu sách đã tải, tồn kho, cảnh báo sắp hết và giá trị kho.
- Danh sách sách dùng cursor pagination.
- Tìm sách đã tải theo tên, tác giả hoặc ISBN.
- Thêm, sửa, xóa sách và cập nhật trực tiếp storefront.
- Danh sách khách hàng dùng cursor pagination; hỗ trợ tìm kiếm, cập nhật tên và xoá tài khoản.
- Hiển thị `trace_id` khi backend trả lỗi để tra log nhanh.
- Responsive cho desktop, tablet và mobile.

Backend hiện chưa có API quản trị đơn hàng. Xoá khách hàng dùng transactional outbox: Auth Service xoá account và tạo event trong một transaction, sau đó RabbitMQ Worker xoá profile ở User Service theo cách idempotent.

## Chạy local

Backend infrastructure và năm Go process cần chạy trước. Trong thư mục này:

```bash
pnpm install
pnpm dev
```

Mở `http://localhost:5174`.

File `.env` local:

```env
VITE_API_BASE_URL=http://localhost:8080
VITE_API_TIMEOUT_MS=10000
VITE_GOOGLE_CLIENT_ID=your-web-client-id.apps.googleusercontent.com
VITE_STOREFRONT_URL=http://localhost:5173
```

`.env` không được commit; `.env.example` được dùng làm mẫu. Admin portal không tạo account khi đăng nhập Google và vẫn kiểm tra role `admin` từ access token trước khi mở back-office.

## Cấp quyền admin local

Tài khoản đăng ký từ storefront mặc định chỉ có role `customer`. Cấp thêm role admin:

```bash
cd ../backend
docker compose exec -T postgres psql -U bookstore -d bookstore \
  -c "UPDATE auth.accounts SET roles = ARRAY['customer','admin'] WHERE email = 'YOUR_EMAIL';"
```

Sau đó đăng xuất/đăng nhập lại để JWT mới chứa role `admin`.

Frontend chỉ decode JWT để điều khiển giao diện. Các endpoint `/api/v1/admin/*` vẫn được backend kiểm tra role; không thể vượt authorization bằng cách sửa frontend.

## Kiểm tra trước khi commit

```bash
pnpm check
```

Lệnh này chạy Prettier check, TypeScript, ESLint, Vitest và production build.

## Production image

```bash
make docker-build
make docker-run
```

Production image dùng Nginx với SPA fallback và cache immutable cho static assets.
