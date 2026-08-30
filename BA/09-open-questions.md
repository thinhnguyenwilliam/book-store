# Câu hỏi nghiệp vụ còn mở

Các mục dưới đây chưa có quyết định đầy đủ trong source. Chúng **không phải yêu cầu đã duyệt**.

## Ưu tiên cao trước production

### Xóa khách hàng và retention

- Xóa account có phải anonymize order, payment, comment, chat và notification không?
- Dữ liệu tài chính cần giữ bao lâu theo pháp lý/kế toán?
- Comment/message sau khi account bị xóa hiển thị tên gì?

### Fulfillment

- Order hiện chưa có địa chỉ giao hàng, phí vận chuyển, trạng thái đóng gói/giao hàng/hoàn hàng.
- Ai xác nhận đã giao và khi nào tiền seller được settlement?
- Chính sách hủy/hoàn tiền sau khi order `confirmed` là gì?

### Tiền tệ và đơn vị tiền

- Field dùng hậu tố `*_cents` nhưng currency mặc định là VND. Có xác nhận rằng một đơn vị lưu trữ tương ứng một VND hay vẫn là 1/100 currency?
- Hệ thống có hỗ trợ nhiều currency không, hay khóa VND cho phase hiện tại?

### Seller

- Ai được tạo seller wallet và gán `seller_id` cho sách?
- Seller có portal riêng, lịch settlement và quyền xem order/allocation không?
- Platform fee có cố định toàn hệ thống hay theo seller/category/campaign?

## Ưu tiên trung bình

### Catalog

- Có cần category, publisher, description, cover image, edition, language và search không?
- Quy tắc ISBN-10/ISBN-13 có cần validate checksum thay vì chỉ kiểm tra không rỗng/unique?
- Xóa sách có lịch sử reservation hiện bị chặn; có cần trạng thái archive/unpublished để admin ngừng bán?

### Comment

- Comment có bắt buộc người mua đã mua sách hay mọi customer đều được bình luận?
- Có cần rating sao, report abuse, moderation reason và audit log?
- Comment hidden có hiển thị placeholder cho reply tree không?

### Chat

- Ai và bằng thao tác nào đóng/mở lại conversation?
- Admin conversation assignment, SLA trả lời và trạng thái agent có cần không?
- Có cần attachment, image, typing indicator và presence không?
- Message được phép sửa trong bao lâu?

### Notification

- Event nào phải gửi email, push hoặc chỉ in-app?
- Người dùng có notification preference và quiet hours không?
- Delivery hết retry cần dead-letter queue, alert hay nút retry thủ công?

## Ưu tiên thấp / roadmap

- Promotion, coupon, tax và recommendation.
- Wishlist, review helpful vote và rating aggregate.
- Inventory nhập kho, nhiều warehouse và backorder.
- Admin màn hình quản lý order, payment, settlement và outbox/DLQ.
- Audit trail cho thao tác nhạy cảm của admin.
- SLO chính thức theo endpoint, tải dự kiến và kế hoạch capacity.

## Cách chốt một câu hỏi

Khi Product Owner duyệt một quyết định:

1. Gán mã `FR-*` hoặc `BR-*` mới.
2. Viết acceptance criteria có case thành công, lỗi và phân quyền.
3. Cập nhật API traceability nếu có endpoint/query/event mới.
4. Cập nhật migration/data dictionary nếu thay đổi dữ liệu.
5. Chỉ sau đó triển khai code và E2E test.
