# Yêu cầu chức năng

Mã yêu cầu là định danh ổn định để liên kết business rule, API và test case.

## Authentication và account

- `FR-AUTH-001` — Khách có thể đăng ký bằng email, mật khẩu và tên hiển thị.
- `FR-AUTH-002` — Người dùng có thể đăng nhập bằng email và mật khẩu.
- `FR-AUTH-003` — Storefront có thể đăng nhập hoặc tạo customer bằng Google GIS.
- `FR-AUTH-004` — Storefront có thể đăng nhập hoặc tạo customer bằng Facebook SDK.
- `FR-AUTH-005` — Admin portal chỉ dùng social login để đăng nhập account đã tồn tại; không tự tạo admin.
- `FR-AUTH-006` — Client có thể làm mới access token bằng refresh cookie HttpOnly.
- `FR-AUTH-007` — Người dùng có thể logout và thu hồi refresh session hiện tại.
- `FR-AUTH-008` — Backend phải phân tách authentication và authorization theo role.

## Hồ sơ và quản lý khách hàng

- `FR-USER-001` — Người dùng xem hồ sơ của chính mình.
- `FR-USER-002` — Người dùng đổi tên hiển thị của chính mình.
- `FR-USER-003` — Admin phân trang danh sách khách hàng bằng opaque cursor.
- `FR-USER-004` — Admin xem và cập nhật hồ sơ khách hàng.
- `FR-USER-005` — Admin gửi yêu cầu xóa account; profile được xóa bất đồng bộ, idempotent qua event.

## Catalog và tồn kho

- `FR-BOOK-001` — Mọi actor xem danh sách sách bằng cursor pagination.
- `FR-BOOK-002` — Mọi actor xem chi tiết một sách.
- `FR-BOOK-003` — Admin tạo sách với title, author, ISBN, giá, tồn kho và seller tùy chọn.
- `FR-BOOK-004` — Admin cập nhật sách.
- `FR-BOOK-005` — Admin xóa sách khi không có lịch sử stock reservation ngăn cản thao tác.
- `FR-BOOK-006` — Order Service có thể reserve, commit hoặc release stock theo idempotency key.

## Giỏ hàng và đơn hàng

- `FR-CART-001` — Customer xem các item trong giỏ của mình.
- `FR-CART-002` — Customer thêm một sách và số lượng vào giỏ.
- `FR-CART-003` — Customer cập nhật số lượng item.
- `FR-CART-004` — Customer xóa một hoặc nhiều item.
- `FR-ORDER-001` — Customer tạo order từ snapshot hiện tại của cart bằng idempotency key.
- `FR-ORDER-002` — Hệ thống reserve toàn bộ stock trước khi order sẵn sàng thanh toán.
- `FR-ORDER-003` — Customer xem danh sách và chi tiết order của mình.
- `FR-ORDER-004` — Customer hủy order khi order còn ở trạng thái cho phép.
- `FR-ORDER-005` — Hệ thống tự reconcile order treo, release stock hoặc thực hiện compensation.

## Ví và thanh toán

- `FR-PAY-001` — Customer tạo và xem ví của chính mình.
- `FR-PAY-002` — Admin điều chỉnh số dư ví với lý do bắt buộc.
- `FR-PAY-003` — Customer tạo payment cho order của mình bằng `wallet` hoặc `vnpay`.
- `FR-PAY-004` — Customer xem payment theo payment ID hoặc order ID.
- `FR-PAY-005` — Hệ thống phân bổ tiền cho seller và platform theo phí cấu hình.
- `FR-PAY-006` — VNPAY IPN chỉ cập nhật payment sau khi chữ ký, reference và số tiền hợp lệ.
- `FR-PAY-007` — Hệ thống đối soát payment pending/refund pending với provider.
- `FR-PAY-008` — Khi bước sau thanh toán thất bại, hệ thống refund/compensate idempotent.

## Bình luận

- `FR-CMT-001` — Mọi actor xem root comments của sách và replies theo opaque cursor.
- `FR-CMT-002` — Customer tạo bình luận gốc hoặc reply một bình luận published.
- `FR-CMT-003` — Tác giả sửa bình luận published của mình.
- `FR-CMT-004` — Tác giả hoặc admin soft-delete bình luận.
- `FR-CMT-005` — Admin chuyển bình luận chưa bị xóa giữa `published` và `hidden`.

## Chat hỗ trợ

- `FR-CHAT-001` — Customer mở hoặc lấy lại support conversation đang open.
- `FR-CHAT-002` — Customer chỉ xem conversation của mình; admin xem conversation được phép quản trị.
- `FR-CHAT-003` — Thành viên gửi text message idempotent bằng `client_message_id`.
- `FR-CHAT-004` — Người gửi sửa/xóa message của mình; admin có thể xóa nội dung vi phạm.
- `FR-CHAT-005` — Thành viên phân trang lịch sử và cập nhật read cursor.
- `FR-CHAT-006` — Client lấy unread count và nhận event realtime qua WebSocket ticket ngắn hạn.

## Notification

- `FR-NOTI-001` — Customer xem notification và unread count của mình.
- `FR-NOTI-002` — Customer đánh dấu một hoặc tất cả notification là đã đọc.
- `FR-NOTI-003` — Client đăng ký hoặc hủy thiết bị nhận push theo application/platform.
- `FR-NOTI-004` — Payment event tạo notification in-app và delivery email/push phù hợp.
- `FR-NOTI-005` — SMTP/FCM tạm lỗi không được NACK domain event gây retry nóng; delivery được retry riêng có delay và giới hạn.

## GraphQL aggregation

- `FR-GQL-001` — Trang chi tiết sách lấy Book và trang Comment đầu tiên trong một query public.
- `FR-GQL-002` — Admin dashboard lấy snapshot catalog và customer trong một query chỉ dành cho admin.
- `FR-GQL-003` — Customer lấy order và payment liên quan trong một query có kiểm tra ownership.
- `FR-GQL-004` — GraphQL chỉ hỗ trợ query; command tiếp tục đi qua REST.
