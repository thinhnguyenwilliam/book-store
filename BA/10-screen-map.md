# Bản đồ màn hình

## Storefront

### `/` — Trang chủ

- Actor: mọi người.
- Mục đích: điểm vào storefront và giới thiệu catalog.
- Dữ liệu chính: danh sách sách nổi bật/gần đây theo implementation của trang.

### `/sach` — Catalog

- Actor: mọi người.
- Mục đích: duyệt danh sách sách bằng cursor pagination.
- API chính: `GET /api/v1/books`.

### `/sach/:id` — Chi tiết sách

- Actor: mọi người; đăng nhập khi muốn bình luận hoặc thêm giỏ.
- Mục đích: xem sách và thread bình luận.
- API chính: GraphQL `bookDetail`; REST command cho comment/cart.

### `/gio-hang` — Giỏ hàng

- Actor nghiệp vụ: customer.
- Mục đích: xem, thêm, đổi số lượng, xóa item và bắt đầu checkout.
- Lưu ý: router hiện không gắn `requiresAuth`; page/API phải xử lý trạng thái chưa đăng nhập. Đây là điểm cần E2E test để tránh màn hình lỗi khi khách vãng lai mở URL trực tiếp.

### `/don-hang/:id` — Chi tiết đơn hàng

- Actor: customer sở hữu order.
- Route guard: bắt buộc đăng nhập.
- Dữ liệu chính: GraphQL `orderDetail` ghép order và payment.

### `/thanh-toan/ket-qua` — Kết quả thanh toán

- Actor: customer.
- Route guard: bắt buộc đăng nhập.
- Mục đích: hiển thị kết quả browser redirect; frontend vẫn phải đọc trạng thái payment từ backend, không tự tin tưởng query string của provider.

### `/dang-nhap` và `/dang-ky`

- Actor: khách chưa đăng nhập.
- Route guard: user đã đăng nhập được chuyển về profile.
- Hỗ trợ password, Google và Facebook theo cấu hình provider.

### `/tai-khoan` — Hồ sơ

- Actor: customer/admin đã đăng nhập.
- Mục đích: xem/cập nhật tên hiển thị và trạng thái phiên.

### Thành phần dùng chung chưa có route riêng

- Chat hỗ trợ được tích hợp dạng widget/panel thay vì trang độc lập.
- Notification in-app/push được tích hợp ở layout/header thay vì trang độc lập.

## Admin portal

### `/dang-nhap` — Đăng nhập quản trị

- Actor: account có `admin.access`.
- Password/social login không tự tạo admin.
- Account không có `admin.access` bị từ chối sau khi backend xác thực.
- Sau đăng nhập, portal mở trang đầu tiên mà account đủ quyền (dashboard / sách / khách hàng / chat / phân quyền).

### `/` — Dashboard

- Actor: account có `analytics.read`, `books.read` và `customers.read`.
- Dữ liệu chính: GraphQL `adminDashboard`.
- Các số liệu là snapshot của cursor page đã tải, không phải global totals toàn database.
- Thiếu một trong các permission trên thì router chuyển sang trang được phép khác.

### `/sach` — Quản lý sách

- Actor: `books.read`; nút tạo/sửa/xóa theo `books.create|update|delete`.

### `/khach-hang` — Quản lý khách hàng

- Actor: `customers.read`; sửa/xóa theo `customers.update|delete`.
- Drawer có thể gán role khi có `roles.read` / `users.assign_roles`.

### `/tro-chuyen` — Trò chuyện hỗ trợ

- Actor: `chat.read`; gửi tin theo `chat.reply`.

### `/phan-quyen` — Vai trò và phân quyền

- Actor: `roles.read`; tạo/sửa/xóa role tùy chỉnh cần `roles.manage`; gán role cần `users.assign_roles`.

### `/khong-co-quyen`

- Actor: đã vào portal nhưng thiếu permission của route đích.

## Khoảng trống UI so với backend

Backend đã có nhưng chưa thấy route quản trị chuyên biệt cho:

- Moderate bình luận.
- Quản lý wallet/balance.
- Quản lý order, payment, settlement và reconciliation.
- Theo dõi email/push delivery thất bại, outbox và dead-letter operations.

Các mục này cần Product Owner xác định có đưa vào admin portal hay chỉ vận hành qua công cụ nội bộ trong phase tiếp theo.
