# Phạm vi và actor

## Mục tiêu sản phẩm

Book Store cung cấp một luồng thương mại điện tử cho sách: khách tìm sách, tương tác, tạo giỏ hàng, đặt hàng, thanh toán và nhận hỗ trợ; quản trị viên vận hành catalog, khách hàng và hội thoại từ back-office.

## Actor

### Khách vãng lai

- Xem danh sách và chi tiết sách.
- Xem bình luận công khai.
- Đăng ký hoặc đăng nhập bằng mật khẩu, Google hay Facebook.
- Không được tạo giỏ hàng, đơn hàng, bình luận hoặc chat trước khi xác thực.

### Khách hàng (`customer`)

- Thực hiện toàn bộ quyền của khách vãng lai.
- Xem và cập nhật hồ sơ của chính mình.
- Quản lý giỏ hàng của chính mình.
- Tạo, xem và hủy đơn của chính mình theo trạng thái cho phép.
- Thanh toán bằng ví nội bộ hoặc VNPAY khi provider được cấu hình.
- Tạo/sửa/xóa bình luận của mình theo business rule.
- Mở hội thoại hỗ trợ, gửi/sửa/xóa tin nhắn của mình và đánh dấu đã đọc.
- Xem thông báo, đánh dấu đã đọc và đăng ký thiết bị nhận FCM.

### Quản trị viên (`admin`)

- Chỉ account có role `admin` mới vào được admin portal.
- Xem dashboard tổng hợp catalog và khách hàng.
- Tạo, sửa, xóa sách.
- Xem, sửa hồ sơ và yêu cầu xóa khách hàng.
- Điều chỉnh số dư ví bằng một delta có lý do và idempotency key.
- Ẩn/hiện hoặc xóa bình luận.
- Xem các cuộc hội thoại hỗ trợ và tham gia trả lời.

### Hệ thống bên ngoài

- Google Identity Services xác minh danh tính qua ID token.
- Meta/Facebook xác minh access token và cung cấp thông tin identity.
- VNPAY nhận yêu cầu checkout, gửi IPN có chữ ký và cung cấp API đối soát.
- SMTP/Mailpit nhận email; FCM nhận push notification.

### Tác nhân nền

- Worker đọc domain event từ RabbitMQ để tạo/xóa profile và điều phối tác vụ bất đồng bộ.
- Outbox publishers đảm bảo event chỉ được phát sau khi transaction dữ liệu đã commit.
- Reconciliation workers xử lý đơn/thanh toán treo và retry email/push có delay, giới hạn số lần.

## Bounded context và quyền sở hữu dữ liệu

- Auth sở hữu account, identity ngoài, refresh session và auth outbox.
- User sở hữu hồ sơ người dùng.
- Catalog sở hữu sách và stock reservation.
- Order sở hữu cart, order và order snapshot item.
- Payment sở hữu wallet, payment, ledger, webhook, settlement và payment outbox.
- Notification sở hữu notification inbox, in-app notification, email và push delivery.
- Comment sở hữu cây bình luận.
- Chat sở hữu conversation, membership, message và chat outbox.

Local dùng chung một PostgreSQL database nhưng mỗi service chỉ truy cập schema mình sở hữu. ID xuyên service là logical reference và được kiểm tra qua gRPC/event; không tạo foreign key xuyên bounded context.

## Ngoài phạm vi đã triển khai

Những mục sau chưa được xem là chức năng hoàn chỉnh trong code hiện tại:

- Địa chỉ giao hàng, phí giao hàng và theo dõi vận chuyển.
- Mã giảm giá, thuế và khuyến mãi.
- Tìm kiếm full-text, danh mục/genre và recommendation.
- Admin quản lý order/payment qua màn hình chuyên biệt.
- Đóng/mở lại support conversation bằng API public.
- Quản lý role admin qua UI.
- Rating sao tách biệt với nội dung bình luận.
- Quy trình seller onboarding và rút tiền.
