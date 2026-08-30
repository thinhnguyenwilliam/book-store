# Tài liệu Business Analysis — Book Store

Thư mục này mô tả nghiệp vụ **đang được triển khai trong source code** của Book Store. Đây là tài liệu `AS-IS`: dùng để thống nhất giữa Product Owner, BA, developer và tester; không thay thế Swagger, protobuf hay migration.

## Cách đọc

1. [Phạm vi và actor](01-scope-and-actors.md)
2. [Yêu cầu chức năng](02-functional-requirements.md)
3. [Quy tắc nghiệp vụ](03-business-rules.md)
4. [Luồng nghiệp vụ](04-business-flows.md)
5. [Từ điển dữ liệu](05-data-dictionary.md)
6. [Truy vết yêu cầu — API](06-api-traceability.md)
7. [Tiêu chí nghiệm thu](07-acceptance-criteria.md)
8. [Yêu cầu phi chức năng](08-non-functional-requirements.md)
9. [Câu hỏi còn mở](09-open-questions.md)
10. [Bản đồ màn hình](10-screen-map.md)

## Phạm vi hệ thống hiện tại

- Storefront cho khách xem sách, quản lý tài khoản, giỏ hàng, đơn hàng, thanh toán, bình luận và chat hỗ trợ.
- Admin portal cho quản trị viên xem dashboard, quản lý sách, khách hàng, bình luận và hội thoại hỗ trợ.
- Backend gồm API Gateway và các service Auth, User, Book, Order, Payment, Notification, Comment, Chat cùng Worker xử lý event.
- REST là giao diện command chính; GraphQL phục vụ ba màn hình tổng hợp; WebSocket phát sự kiện chat thời gian thực.
- PostgreSQL lưu dữ liệu bền vững, Redis dùng cache/realtime coordination, RabbitMQ truyền domain event theo cơ chế bất đồng bộ.

## Quy ước trạng thái tài liệu

- `Implemented`: đã có route/use case tương ứng trong source.
- `Partial`: backend đã có nhưng UI hoặc quy trình vận hành chưa hoàn chỉnh.
- `Proposed`: chỉ là đề xuất, chưa được xem là yêu cầu chính thức.
- `Open`: cần Product Owner quyết định.

Mọi thay đổi nghiệp vụ nên cập nhật yêu cầu, business rule, acceptance criteria và traceability trong cùng pull request với code.

## Nguồn sự thật kỹ thuật

- HTTP routes: [`backend/internal/gateway/http/handler.go`](../backend/internal/gateway/http/handler.go)
- HTTP DTO: [`backend/internal/gateway/http/dto.go`](../backend/internal/gateway/http/dto.go)
- gRPC contracts: [`backend/api/proto/bookstore/v1`](../backend/api/proto/bookstore/v1)
- GraphQL schema: [`backend/internal/gateway/graphql/schema.graphqls`](../backend/internal/gateway/graphql/schema.graphqls)
- Database migrations: [`backend/migrations`](../backend/migrations)
- Storefront routes: [`storefront/src/app/router/index.ts`](../storefront/src/app/router/index.ts)
- Admin routes: [`admin-portal/src/app/router/index.ts`](../admin-portal/src/app/router/index.ts)

Tài liệu được đối chiếu với source ngày **30/08/2026**.
