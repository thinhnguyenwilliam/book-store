# Yêu cầu phi chức năng

## Bảo mật

- `NFR-SEC-001` — Password chỉ lưu hash; secret provider, JWT key và payment secret không được commit Git hoặc trả xuống frontend.
- `NFR-SEC-002` — Access token ngắn hạn; refresh token opaque chỉ nằm trong cookie HttpOnly và hash trong database.
- `NFR-SEC-003` — Production bắt buộc HTTPS, cookie `Secure`, origin/redirect URI allowlist chính xác, không wildcard.
- `NFR-SEC-004` — Authentication và authorization là hai lớp riêng; mọi admin endpoint kiểm tra role ở server.
- `NFR-SEC-005` — Webhook thanh toán kiểm tra chữ ký bằng constant-time comparison và chống replay/deduplicate.
- `NFR-SEC-006` — Log không chứa password, JWT, refresh token, provider credential hoặc request body nhạy cảm.
- `NFR-SEC-007` — GraphQL có giới hạn body, token, depth và complexity; introspection tắt mặc định production.

## Hiệu năng

- `NFR-PERF-001` — Mục tiêu quan sát cho API thông thường là dưới 200 ms, không phải hard timeout tuyệt đối.
- `NFR-PERF-002` — HTTP request budget mặc định 2 giây; unary gRPC dùng deadline nhỏ hơn hoặc bằng deadline còn lại.
- `NFR-PERF-003` — Danh sách tăng trưởng lớn dùng cursor pagination và limit tối đa 100.
- `NFR-PERF-004` — Book và cart có Redis cache với TTL/invalidation; cache lỗi phải fallback database.
- `NFR-PERF-005` — Cache miss đồng thời phải hạn chế stampede bằng lock/singleflight.

## Tin cậy và nhất quán

- `NFR-REL-001` — Giao dịch xuyên service dùng Saga, idempotency và compensation, không giả định distributed transaction.
- `NFR-REL-002` — Domain event quan trọng dùng transactional outbox; consumer dùng inbox/idempotent handling.
- `NFR-REL-003` — Retry phải có delay, giới hạn attempts và lưu last error; không retry nóng vô hạn.
- `NFR-REL-004` — Payment cần reconciliation vì timeout có thể khiến trạng thái local không rõ dù provider đã xử lý.
- `NFR-REL-005` — Service hỗ trợ graceful shutdown cho HTTP, gRPC, outbox và worker.

## Quan sát hệ thống

- `NFR-OBS-001` — Mọi HTTP response có request ID; trace ID được truyền qua HTTP, gRPC, outbox và RabbitMQ.
- `NFR-OBS-002` — Local log dùng thời gian dễ đọc; production JSON dùng RFC3339 kèm timezone.
- `NFR-OBS-003` — Log file theo service/ngày, rotate lúc 00:00, giới hạn size và retention.
- `NFR-OBS-004` — Payment mismatch, delivery hết retry và outbox không publish được phải có khả năng truy vết vận hành.

## Khả năng bảo trì

- `NFR-MAIN-001` — Domain/application không phụ thuộc Echo, gRPC generated model, GORM, Viper hoặc JWT implementation.
- `NFR-MAIN-002` — HTTP DTO, protobuf và domain model được map rõ ràng tại boundary.
- `NFR-MAIN-003` — Mỗi service chỉ đọc/ghi schema do nó sở hữu.
- `NFR-MAIN-004` — CI phải kiểm tra backend, storefront, admin portal và generated Swagger/GraphQL/protobuf không lệch source.

## Khả năng truy cập và trải nghiệm

- `NFR-UX-001` — Lỗi từ Google/Facebook được chuẩn hóa thành code/provider/retryable; không hiển thị raw provider error.
- `NFR-UX-002` — UI không phụ thuộc nội dung opaque cursor và xử lý đúng trang cuối.
- `NFR-UX-003` — Kết nối chat realtime mất không làm mất lịch sử; client phải có thể tải bù qua REST.
