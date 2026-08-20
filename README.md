# Book Store

Book Store gồm backend Golang theo hướng microservice, Clean Architecture và DDD cùng storefront Vue 3 + TypeScript. Gateway public dùng Echo/HTTP, các service nội bộ giao tiếp bằng gRPC, dữ liệu lưu trong PostgreSQL qua GORM và domain event được xử lý bằng transactional outbox + RabbitMQ.

Hiện repository đã có `backend/` và `storefront/`. `admin-portal/` sẽ được bổ sung sau.

## Kiến trúc

```text
Storefront / Admin Portal
           |
           | HTTP/JSON :8080
           v
      Echo API Gateway
        /     |      \
      gRPC   gRPC    gRPC
      /       |        \
 Auth      User       Book
  |         ^           |
  | outbox  | gRPC      |
  v         |           |
RabbitMQ -> Worker       |
   \         |         /
    PostgreSQL / bookstore
      |       |       |
    auth    users   catalog
```

Redis được giữ riêng cho cache/rate-limit. RabbitMQ chịu trách nhiệm truyền domain event.

Auth dùng access token JWT ngắn hạn `5m` và refresh token opaque `168h`. Storefront chỉ giữ access token trong memory; refresh token được Gateway đặt trong cookie `HttpOnly` và rotate qua PostgreSQL mỗi lần làm mới phiên.

## Yêu cầu

Để chạy toàn bộ hệ thống, máy cần có:

- Git.
- Docker Engine hoặc Docker Desktop.
- Docker Compose v2 (`docker compose`).
- GNU Make nếu muốn dùng các lệnh `make`.

Không cần cài Go, PostgreSQL, Redis hay RabbitMQ nếu chạy bằng Docker Compose.

Kiểm tra Docker:

```bash
docker --version
docker compose version
docker ps
```

## Chạy nhanh bằng Docker Compose

Clone repository:

```bash
git clone https://github.com/thinhnguyenwilliam/book-store.git
cd book-store/backend
```

Build và chạy toàn bộ stack ở background:

```bash
make up
```

Nếu đã có PostgreSQL volume từ phiên bản cũ, áp migration mới trước khi thử auth:

```bash
make migrate
```

Nếu máy không có `make`, dùng trực tiếp:

```bash
docker compose up -d --build
```

Lần chạy đầu Docker cần tải image và build năm Go service nên có thể mất vài phút.

Kiểm tra container:

```bash
make ps
```

Kiểm tra API Gateway:

```bash
curl http://localhost:8080/healthz
```

Kết quả mong đợi:

```json
{"status":"ok"}
```

## Chạy Go service trực tiếp trên máy

Chỉ khởi động PostgreSQL, pgAdmin, Redis, RedisInsight và RabbitMQ bằng Docker; năm service Go trong Docker sẽ được dừng để giải phóng port:

```bash
cd backend
make local-prepare
```

Sau đó mở năm terminal và chạy từng service:

```bash
make local-auth
make local-user
make local-book
make local-worker
make local-gateway
```

Muốn hot reload trên máy, thay `local-*` bằng `watch-*`, ví dụ `make watch-auth`. Air được cài riêng vào `backend/.tools`, không cần cài global và không build Docker image. Các service local dùng [backend/config/local.yml](backend/config/local.yml) với hostname `localhost`.

## Chạy toàn bộ bằng Docker với Air hot reload

Chạy development stack có Air:

```bash
cd backend
make dev
```

Theo dõi log reload của năm Go service:

```bash
make dev-logs
```

Khi sửa file `.go`, `.toml`, `.yaml` hoặc `.yml`, Air tự build và restart service. Do backend đang dùng một Go module chung, thay đổi source có thể làm nhiều service build lại.

Xem trạng thái hoặc dừng development stack:

```bash
make dev-ps
make dev-down
```

`make up` dùng image production-like, không mount source và không hot reload. `make dev` dùng [backend/Dockerfile.dev](backend/Dockerfile.dev), [backend/.air.toml](backend/.air.toml) và [backend/docker-compose.dev.yml](backend/docker-compose.dev.yml).

Các Go process hỗ trợ graceful shutdown cho HTTP, gRPC, outbox và RabbitMQ worker. Timeout nội bộ được cấu hình bằng `shutdown.timeout` (`12s` ở local); Air và Docker chờ `15s` trước khi force kill.

## Chạy storefront Vue trên máy

Sau khi Gateway chạy tại `http://localhost:8080`, mở terminal mới:

```bash
cd storefront
cp .env.example .env
pnpm install
pnpm dev
```

Mở <http://localhost:5173>. Source được hot reload bởi Vite. Chạy toàn bộ type-check, lint, unit test và production build bằng:

```bash
pnpm check
```

Xem thêm cấu hình `.env`, Docker/Nginx và cấu trúc source tại [storefront/README.md](storefront/README.md).

## Địa chỉ dịch vụ

- API Gateway: <http://localhost:8080>
- Storefront: <http://localhost:5173>
- Swagger UI: <http://localhost:8080/swagger/index.html>
- PostgreSQL: `localhost:5432`
- pgAdmin: <http://localhost:5050>
- Redis: `localhost:6379`
- RedisInsight: <http://localhost:5540>
- RabbitMQ AMQP: `localhost:5672`
- RabbitMQ Management: <http://localhost:15672>

Thông tin đăng nhập local:

- pgAdmin: `admin@bookstore.com` / `admin`
- RabbitMQ Management: `bookstore` / `bookstore`
- PostgreSQL: database/user/password đều là `bookstore`

Các credentials này chỉ phục vụ local development, không được sử dụng ở production.

## Thử API

Cách dễ nhất là mở Swagger UI:

```text
http://localhost:8080/swagger/index.html
```

Hoặc đăng ký tài khoản bằng `curl`:

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"reader@example.com","password":"password123","display_name":"Reader"}'
```

Đăng nhập:

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"reader@example.com","password":"password123"}'
```

Profile được tạo bất đồng bộ qua RabbitMQ. Ngay sau khi register có thể có độ trễ ngắn trước khi endpoint `/api/v1/users/me` trả về profile.

## Quản lý chương trình

Xem tất cả lệnh:

```bash
make help
```

Theo dõi log:

```bash
make logs
make local-prepare
make dev
make dev-logs
make dev-down
```

Chỉ xem log Gateway và worker:

```bash
docker compose logs -f --tail=200 gateway worker-service
```

Khởi động lại:

```bash
make restart
```

Dừng chương trình nhưng giữ dữ liệu:

```bash
make down
```

Dừng và xóa toàn bộ database/queue/cache local:

```bash
make down-volumes
```

`make down-volumes` xóa Docker volumes và không thể khôi phục dữ liệu local đã lưu.

## Lệnh dành cho phát triển

Chạy test, vet, build và kiểm tra Docker Compose:

```bash
make check
```

Sinh lại protobuf và Swagger:

```bash
make generate
```

Hoặc chạy riêng:

```bash
make proto
make swagger
```

Format source:

```bash
make fmt
```

Các tool như Buf, protobuf plugins và Swag được cài vào `backend/.tools`; Go build cache nằm trong `backend/.cache`. Hai thư mục này đã được Git ignore.

## Cấu hình

Các service đọc [backend/config/config.yml](backend/config/config.yml) bằng Viper. Project không dùng `.env`.

Docker Compose mount file này read-only vào từng Go container:

```text
/service -config /app/config.yml
```

Không ghi secret production vào file config đang commit. Khi deploy thật, hãy mount config riêng từ Docker/Kubernetes Secret hoặc secret manager.

## Xử lý lỗi thường gặp

Nếu `docker compose up` báo port đang được sử dụng, kiểm tra các port `8080`, `5050`, `5432`, `5540`, `5672`, `6379` và `15672`, sau đó dừng process/container đang chiếm port hoặc đổi mapping trong `backend/docker-compose.yml`.

Nếu service chưa sẵn sàng:

```bash
make ps
make logs
```

Nếu migration hoặc dữ liệu local bị lỗi và không cần giữ dữ liệu:

```bash
make down-volumes
make up
```

Nếu Docker báo permission denied, đảm bảo tài khoản hiện tại có quyền dùng Docker hoặc Docker Desktop đã được khởi động.

## Cấu trúc repository

```text
Book-store/
├── backend/
│   ├── api/proto/       # gRPC contracts
│   ├── cmd/             # gateway và service entrypoints
│   ├── config/          # Viper YAML config
│   ├── docs/            # generated Swagger specification
│   ├── gen/             # generated protobuf Go code
│   ├── internal/        # domain, application, adapter, delivery
│   ├── migrations/      # PostgreSQL bootstrap migrations
│   ├── docker-compose.yml
│   └── Makefile
├── storefront/          # planned
└── admin-portal/        # planned
```

Tài liệu kiến trúc backend chi tiết nằm tại [backend/README.md](backend/README.md).
