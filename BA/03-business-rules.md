# Quy tắc nghiệp vụ

## Account và phiên đăng nhập

- `BR-AUTH-001` — Email được trim, chuyển lowercase và phải hợp lệ; email là duy nhất.
- `BR-AUTH-002` — Mật khẩu đăng ký có tối thiểu 8 ký tự.
- `BR-AUTH-003` — Tên hiển thị không dài quá 100 ký tự.
- `BR-AUTH-004` — Account mới luôn nhận role `customer`; không có public API tự cấp role `admin`.
- `BR-AUTH-005` — Access token mặc định sống 5 phút. Refresh session mặc định sống 168 giờ và chỉ lưu hash token trong database.
- `BR-AUTH-006` — Refresh token được rotate. Phát hiện reuse phải thu hồi session family liên quan.
- `BR-AUTH-007` — Social login phải kiểm tra provider state chống CSRF. Google còn phải kiểm tra nonce của ID token.
- `BR-AUTH-008` — Google/Facebook identity dùng cặp `provider + subject` làm định danh ổn định; không dùng email làm identity key.
- `BR-AUTH-009` — Social account chỉ được tạo khi `create_account=true`; admin portal gửi `false`.
- `BR-AUTH-010` — Provider credential không được lưu hoặc ghi log.

## Catalog và tồn kho

- `BR-BOOK-001` — Title, author và ISBN không được rỗng; ISBN là duy nhất.
- `BR-BOOK-002` — Giá và tồn kho không được âm. Đơn vị giá là đơn vị nhỏ nhất của currency, tên field hiện tại là `*_cents`.
- `BR-BOOK-003` — Mỗi reservation dùng idempotency key duy nhất; một order chỉ có một reservation cho mỗi book.
- `BR-BOOK-004` — Số lượng reserve phải dương và không vượt tồn kho khả dụng.
- `BR-BOOK-005` — Reservation có trạng thái `reserved`, `committed`, `released`; transition sai trạng thái bị từ chối.
- `BR-BOOK-006` — Sách có lịch sử stock reservation không được hard-delete.

## Cart và order

- `BR-CART-001` — Mỗi customer chỉ có một cart item cho một book.
- `BR-CART-002` — Số lượng mỗi cart item nằm trong khoảng 1–100.
- `BR-ORDER-001` — Không thể tạo order từ cart rỗng.
- `BR-ORDER-002` — `Idempotency-Key` là bắt buộc khi tạo order và khi thanh toán; cùng customer + key không tạo bản ghi đôi.
- `BR-ORDER-003` — Order item lưu snapshot title, seller, giá và số lượng tại thời điểm tạo order; thay đổi Book sau đó không làm đổi order cũ.
- `BR-ORDER-004` — Tổng order bằng tổng `unit_price × quantity` của các snapshot item và phải lớn hơn 0.
- `BR-ORDER-005` — Stock reservation mặc định hết hạn sau 15 phút.
- `BR-ORDER-006` — Luồng trạng thái chính: `pending → stock_reserved → payment_pending → confirmed`.
- `BR-ORDER-007` — Order có thể sang `cancelled` khi user hủy, reserve hết hạn hoặc payment thất bại.
- `BR-ORDER-008` — `compensation_pending` được dùng khi đã thanh toán nhưng commit stock thất bại; hệ thống phải refund rồi release stock trước khi kết thúc ở `cancelled`.
- `BR-ORDER-009` — Customer chỉ hủy trực tiếp khi order đang `pending` hoặc `stock_reserved`; hủy lặp lại là no-op thành công.

## Payment và ledger

- `BR-PAY-001` — Mỗi order có tối đa một payment; mỗi buyer + idempotency key là duy nhất.
- `BR-PAY-002` — Payment amount phải dương và bằng tổng order.
- `BR-PAY-003` — Trạng thái payment gồm `pending`, `succeeded`, `failed`, `refund_pending`, `refunded`.
- `BR-PAY-004` — Wallet payment chỉ thành công khi buyer đủ tiền, trừ wallet được phép âm.
- `BR-PAY-005` — Ledger transaction có idempotency key duy nhất. Tổng các ledger entry của một transaction phải cân bằng về 0.
- `BR-PAY-006` — Mỗi seller nhận allocation trừ phí platform; platform nhận tổng phí. Local config hiện là 1000 basis points, tương đương 10%.
- `BR-PAY-007` — VNPAY return URL chỉ phục vụ trải nghiệm browser; nguồn xác nhận tin cậy là IPN đã xác minh hoặc kết quả đối soát server-to-server.
- `BR-PAY-008` — Webhook event được deduplicate theo `provider + event_id`.
- `BR-PAY-009` — Provider transaction ID, khi có, là duy nhất trong provider.

## Comment

- `BR-CMT-001` — Nội dung sau trim dài 1–2000 ký tự Unicode.
- `BR-CMT-002` — Cây comment dùng adjacency list (`parent_id`) kết hợp `root_id` và `depth`, không dùng nested set.
- `BR-CMT-003` — Root có `parent_id=null`, `root_id=id`, `depth=0`; reply tối đa depth 3.
- `BR-CMT-004` — Reply phải cùng book với parent và parent phải ở trạng thái `published`.
- `BR-CMT-005` — Chỉ tác giả được sửa comment của mình và comment phải còn `published`.
- `BR-CMT-006` — Xóa là soft-delete. Comment `deleted` không được sửa hoặc moderate trở lại.
- `BR-CMT-007` — Admin chỉ moderate sang `published` hoặc `hidden`.

## Chat

- `BR-CHAT-001` — Mỗi customer chỉ có một support conversation `open` tại một thời điểm.
- `BR-CHAT-002` — Message hiện chỉ hỗ trợ loại `text`, dài 1–4000 ký tự Unicode.
- `BR-CHAT-003` — `sender_id + client_message_id` là duy nhất để chống gửi trùng khi retry/reconnect.
- `BR-CHAT-004` — Sequence number tăng trong phạm vi conversation và là duy nhất.
- `BR-CHAT-005` — Message được soft-delete; read cursor không được âm và không lùi ngược.
- `BR-CHAT-006` — WebSocket không nhận access token dài hạn trực tiếp trong URL; client xin ticket ngắn hạn qua API đã xác thực.

## Notification

- `BR-NOTI-001` — Một event chỉ tạo tối đa một in-app notification cho một user.
- `BR-NOTI-002` — Inbox deduplicate theo `event_id`; RabbitMQ at-least-once không được tạo notification đôi.
- `BR-NOTI-003` — Email delivery duy nhất theo `event_id + recipient`.
- `BR-NOTI-004` — Push delivery duy nhất theo `notification + installation`.
- `BR-NOTI-005` — Email local retry tối đa 10 lần; push tối đa 5 lần. Delivery hết retry giữ trạng thái thất bại để vận hành tra cứu.

## Phân trang

- `BR-PAGE-001` — Books, customers, orders, comments, notifications và chat dùng opaque cursor; client không được tự giải mã hoặc sửa cursor.
- `BR-PAGE-002` — Limit mặc định thường là 20, riêng message là 30; limit tối đa là 100.
- `BR-PAGE-003` — Khi `has_more=false`, response không bắt buộc có `next_cursor`; client không được gọi trang tiếp với cursor rỗng/undefined.
