# Tiêu chí nghiệm thu

Các scenario dưới đây ưu tiên luồng có rủi ro nghiệp vụ cao. Chúng có thể được chuyển trực tiếp thành integration/E2E test.

## AC-AUTH-01 — Đăng ký thành công

- Given email chưa tồn tại, password từ 8 ký tự và display name hợp lệ.
- When khách gửi yêu cầu đăng ký.
- Then API trả access token, đặt refresh cookie HttpOnly và account có role `customer`.
- And event tạo profile được ghi cùng transaction với account.
- And profile cuối cùng được tạo dù RabbitMQ tạm thời gián đoạn.

## AC-AUTH-02 — Refresh rotate và reuse detection

- Given một refresh token còn hạn.
- When client refresh lần đầu.
- Then token cũ bị thay thế và client nhận access/refresh token mới.
- When token cũ bị sử dụng lại.
- Then request bị từ chối và session family liên quan bị thu hồi.

## AC-AUTH-03 — Social login an toàn

- Given provider state/nonce không khớp, hết hạn hoặc thiếu.
- When client gửi Google/Facebook credential hợp lệ.
- Then backend vẫn từ chối trước khi tạo session.
- And response không chứa raw credential hoặc lỗi nội bộ của provider.

## AC-BOOK-01 — Cursor cuối danh sách

- Given số sách không vượt limit.
- When client tải trang đầu.
- Then `has_more=false` và `next_cursor` có thể vắng mặt.
- And UI không gửi request trang tiếp với `cursor=undefined` hoặc cursor rỗng.

## AC-CART-01 — Số lượng hợp lệ

- Given customer đã đăng nhập và book tồn tại.
- When quantity nằm ngoài 1–100.
- Then API trả invalid input và cart không thay đổi.
- When add cùng book lần nữa với quantity hợp lệ.
- Then cart vẫn chỉ có một item cho book đó.

## AC-ORDER-01 — Idempotent create order

- Given cart có item hợp lệ và đủ stock.
- When hai request tạo order dùng cùng user và idempotency key.
- Then chỉ có một order được tạo và không reserve stock hai lần.

## AC-ORDER-02 — Reserve stock thất bại

- Given cart có nhiều book và một book không đủ stock.
- When customer tạo order.
- Then order bị cancel, các reservation đã thành công trước đó được release và không có payment.

## AC-ORDER-03 — Hủy order

- Given order ở `stock_reserved`.
- When owner hủy order.
- Then stock được release và order sang `cancelled`.
- When hủy lại cùng order.
- Then thao tác thành công idempotent.
- Given order đã `confirmed`.
- When owner yêu cầu hủy qua API hiện tại.
- Then request bị từ chối vì trạng thái không hợp lệ.

## AC-PAY-01 — Wallet payment

- Given buyer đủ tiền và order ở `stock_reserved`.
- When customer thanh toán với idempotency key mới.
- Then buyer bị trừ đúng tổng, seller/platform nhận đúng allocation, ledger cân bằng và payment succeeded.
- And retry cùng key không ghi sổ hoặc trừ tiền lần hai.

## AC-PAY-02 — VNPAY IPN

- Given payment pending.
- When IPN có chữ ký sai, reference sai hoặc amount sai.
- Then payment không chuyển sang succeeded.
- When IPN hợp lệ được gửi lặp lại.
- Then chỉ lần đầu áp dụng business transition; các lần sau trả kết quả idempotent.

## AC-PAY-03 — Compensation

- Given payment succeeded nhưng commit stock lỗi.
- When Saga chuyển sang compensation.
- Then order ở `compensation_pending` cho đến khi refund và release stock hoàn tất.
- And retry không refund hai lần.

## AC-CMT-01 — Reply tree

- Given parent published thuộc cùng book và depth dưới 3.
- When customer tạo reply hợp lệ.
- Then reply giữ đúng root ID và depth tăng 1.
- Given parent depth bằng 3 hoặc thuộc book khác.
- Then request bị từ chối và không tạo comment.

## AC-CMT-02 — Quyền sửa/xóa/moderate

- Given comment published.
- When user khác không phải admin sửa/xóa.
- Then request bị từ chối.
- When tác giả soft-delete.
- Then cấu trúc thread còn tồn tại nhưng comment không sửa hoặc publish lại được.

## AC-CHAT-01 — Gửi message idempotent

- Given member của conversation open.
- When cùng sender gửi lại cùng `client_message_id` và cùng nội dung.
- Then không tạo message thứ hai.
- When cùng ID nhưng payload xung đột.
- Then API báo idempotency conflict.

## AC-CHAT-02 — Realtime và lịch sử

- Given client đã lấy WebSocket ticket hợp lệ.
- When message được commit.
- Then receiver nhận event có conversation/message ID và sequence tương ứng.
- And reconnect có thể dùng REST cursor để bù message bị lỡ.
- Given ticket hết hạn hoặc sai.
- Then WebSocket bị từ chối.

## AC-NOTI-01 — Provider email bị gián đoạn

- Given payment event đã được ghi vào notification inbox.
- When SMTP tạm lỗi.
- Then domain event vẫn được ACK, in-app notification không bị tạo đôi và email delivery chuyển failed để retry có delay.
- And delivery dừng retry sau giới hạn cấu hình, không tạo vòng NACK nóng ở RabbitMQ.

## AC-ADMIN-01 — Phân quyền admin

- Given account chỉ có role customer.
- When gọi bất kỳ `/admin/*` hoặc query `adminDashboard`.
- Then server trả forbidden và không thực hiện mutation.

## AC-GQL-01 — Ownership và partial aggregate

- Given customer A yêu cầu `orderDetail` của customer B.
- Then query bị từ chối/not found theo chính sách không lộ dữ liệu.
- Given order hợp lệ chưa có payment.
- Then query vẫn trả order và `payment=null`.
