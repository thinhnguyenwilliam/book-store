# Từ điển dữ liệu

Tài liệu này mô tả ý nghĩa nghiệp vụ của entity. Kiểu dữ liệu và constraint chi tiết lấy từ [`backend/migrations`](../backend/migrations).

## Auth

### `auth.accounts`

Account dùng để xác thực và phân quyền. `id` là UUID dùng xuyên hệ thống; `email` duy nhất; `password_hash` không được trả ra API; `roles` chứa ít nhất role nghiệp vụ như `customer` hoặc `admin`.

### `auth.account_identities`

Liên kết account với identity Google/Facebook. Khóa nghiệp vụ là provider + subject. Một account có thể có nhiều phương thức đăng nhập.

### `auth.refresh_sessions`

Phiên refresh token có token hash, family, thời điểm hết hạn/thu hồi và liên kết token thay thế. Dùng cho rotate và phát hiện reuse.

### `auth.outbox_events`

Domain event của Auth được ghi cùng transaction với aggregate. `available_at`, `attempts`, `published_at`, `last_error` phục vụ retry; `trace_id` nối log bất đồng bộ.

## User

### `users.user_profiles`

Hồ sơ hiển thị gồm cùng `id` với account, email và display name. Auth không query trực tiếp bảng này; profile được tạo/xóa qua event và gRPC.

## Catalog

### `catalog.books`

Sách bán trên storefront. Giá lưu ở `price_cents`, stock là số nguyên không âm, ISBN duy nhất. `seller_id` là logical reference đến chủ sở hữu ví; rỗng thì Order dùng platform owner.

### `catalog.stock_reservations`

Giữ số lượng sách cho một order trong thời hạn checkout. Một order/book chỉ có một reservation. Status cho biết stock đang giữ, đã commit hay đã release.

## Order

### `orders.cart_items`

Giỏ hàng theo user. Khóa duy nhất user/book giúp add cùng sách trở thành cập nhật một item thay vì tạo dòng trùng.

### `orders.orders`

Header đơn hàng, chứa owner, state Saga, tổng tiền, currency, payment logical reference, lỗi gần nhất, idempotency key và hạn reservation.

### `orders.order_items`

Snapshot thương mại của sách lúc đặt hàng. Dữ liệu title, seller và unit price không phụ thuộc thay đổi catalog sau này. Có foreign key nội service về order.

## Payment

### `payments.wallets`

Số dư theo owner và currency. `allow_negative` chỉ dành cho wallet được phép theo chính sách hệ thống.

### `payments.payments`

Một payment duy nhất cho mỗi order. Chứa buyer, amount, platform fee, provider, provider reference/transaction, checkout URL, trạng thái và mốc thời gian thanh toán.

### `payments.payment_allocations`

Phân bổ gross amount của một payment cho từng seller trước khi trừ platform fee.

### `payments.ledger_transactions` và `payments.ledger_entries`

Sổ cái double-entry. Transaction là nghiệp vụ idempotent; entries là các delta wallet phải cân bằng tổng 0.

### `payments.webhook_events`

Lưu webhook/IPN đã nhận để audit và deduplicate. Raw payload là dữ liệu nhạy cảm vận hành, không trả trực tiếp cho frontend.

### `payments.settlement_reconciliations`

Lịch sử so sánh trạng thái/số tiền local với provider. `matched=false` cần cảnh báo hoặc xử lý vận hành.

### `payments.outbox_events`

Phát payment event sau commit để Order và Notification xử lý an toàn.

## Notification

### `notifications.inbox_events`

Consumer inbox để deduplicate domain event theo event ID và ghi lỗi xử lý.

### `notifications.notifications`

Thông báo in-app theo user, có title/body/data và `read_at`.

### `notifications.email_deliveries`

Email job độc lập với domain event. Status gồm `pending`, `sending`, `sent`, `failed`, `skipped`; attempts và last error phục vụ retry/audit.

### `notifications.device_installations`

Thiết bị FCM theo user, app (`storefront`/`admin`) và platform (`web`/`android`/`ios`). Disable token thay vì xóa lịch sử delivery.

### `notifications.push_deliveries`

Một lần gửi notification đến một installation, có retry status và provider message ID.

## Comment

### `comments.comments`

Một bảng lưu cả root và reply. `parent_id` mô tả cạnh trực tiếp; `root_id` gom thread; `depth` giới hạn 0–3. `deleted_at` và status giữ cấu trúc thread khi nội dung bị xóa.

## Chat

### `chat.conversations`

Hội thoại hỗ trợ của customer, chứa status, sequence gần nhất, preview và thời điểm message gần nhất.

### `chat.conversation_members`

Quyền tham gia và read cursor theo user. Foreign key nội service về conversation.

### `chat.messages`

Message text có sender snapshot, client message ID chống trùng, sequence number, edit/delete timestamp.

### `chat.outbox_events`

Event message/conversation được lưu cùng transaction để phát realtime và notification mà không mất sự kiện.

## Quan hệ xuyên service

Các quan hệ sau là logical reference, không có database foreign key:

- Profile ID → Account ID.
- Book seller ID → Wallet owner/account ID.
- Cart/Order user ID → Account/Profile ID.
- Order book ID → Book ID.
- Payment order ID → Order ID.
- Comment book/author ID → Book/Profile ID.
- Chat customer/sender/member ID → Account/Profile ID.
- Notification user ID → Account/Profile ID.

Việc thiếu foreign key ở các quan hệ này là có chủ đích để không cho một service phụ thuộc trực tiếp schema của service khác.
