# Book Store Backend

Backend gồm năm process Go độc lập và một nhóm hạ tầng dùng cho local development:

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
    auth    users   catalog     <- schemas

RabbitMQ xử lý domain event; Redis được giữ riêng cho cache/rate-limit.
```

Gateway là public entry point. Ba service giao tiếp với Gateway bằng gRPC.

## Một database hay ba database?

Local hiện chỉ chạy **một PostgreSQL container và một database `bookstore`**. Database này có ba schema:

- `auth`: do auth-service sở hữu.
- `users`: do user-service sở hữu.
- `catalog`: do book-service sở hữu.

“Service sở hữu dữ liệu” nghĩa là service khác không query hoặc sửa trực tiếp schema đó; nó phải gọi gRPC/API của service sở hữu. Điều này không bắt buộc mỗi service phải có một PostgreSQL server riêng. Khi hệ thống lớn hơn, từng schema có thể được chuyển sang database/server riêng mà không đổi domain và use case.

## Cấu hình Viper

Tất cả Go process đọc [config/config.yml](config/config.yml) bằng Viper. Không dùng `.env`:

```yaml
postgres:
  url: "postgres://bookstore:bookstore@postgres:5432/bookstore?sslmode=disable"

redis:
  address: "redis:6379"
  password: ""
  database: 0

rabbitmq:
  url: "amqp://bookstore:bookstore@rabbitmq:5672/"
  exchange: "bookstore.events"
  user_profile_queue: "user.profile.create"
  account_registered_routing_key: "account.registered"
```

Compose mount file này read-only vào container và truyền đường dẫn bằng CLI flag:

```text
/service -config /app/config.yml
```

File hiện chứa credentials local để chạy nhanh. Không commit secret production; production nên mount một `config.yml` riêng bằng secret manager hoặc Docker/Kubernetes Secret.

## Cấu trúc source

```text
api/proto/                       gRPC contracts
cmd/                             composition root của từng binary
config/config.yml                Viper YAML config
gen/                             Go code sinh từ protobuf
internal/<service>/domain/       entity, business rules, domain errors
internal/<service>/application/  use cases và ports
internal/<service>/adapter/      PostgreSQL/JWT/bcrypt adapters
internal/<service>/delivery/grpc gRPC transport
internal/gateway/http/           Echo handlers và middleware
migrations/                      schema của database bookstore
```

Các PostgreSQL adapter sử dụng GORM. Domain không import Echo, gRPC, GORM, Viper hoặc JWT.

## Transactional outbox giải quyết vấn đề gì?

Luồng cũ có hai thao tác độc lập:

```text
1. auth-service commit account vào PostgreSQL
2. Gateway gọi user-service để tạo profile
```

Nếu bước 1 thành công nhưng bước 2 timeout hoặc user-service bị down, API báo lỗi nhưng account đã tồn tại. Retry register lại gặp `email already exists`, trong khi profile vẫn chưa có. Không thể dùng một transaction database thông thường bao trùm hai microservice.

Luồng mới:

```text
BEGIN
  INSERT auth.accounts
  INSERT auth.outbox_events
COMMIT

outbox dispatcher -> RabbitMQ -> worker-service -> user-service
```

Account và outbox event được GORM ghi trong cùng một PostgreSQL transaction:

- Transaction rollback: không có account và cũng không có event.
- Transaction commit nhưng RabbitMQ down: account và pending event vẫn còn trong PostgreSQL.
- RabbitMQ hoạt động lại: dispatcher tự retry, nhận publisher confirm rồi mới đánh dấu event đã publish.
- Worker chỉ ACK message sau khi user-service tạo profile thành công.
- Dispatcher crash sau khi enqueue nhưng trước khi đánh dấu published: job có thể được giao lại, vì vậy user repository xử lý idempotent theo `user_id`.

Đây là mô hình **at-least-once delivery**: chấp nhận event có thể đến nhiều lần, nhưng không được làm dữ liệu bị nhân đôi.

## Chạy local

```bash
cd ~/WorkSpace/Book-store/backend
make up
```

Lệnh trên tương đương `docker compose up -d --build`. Dùng `make logs` để theo dõi log và `make ps` để xem trạng thái container.

### Air hot reload

Để phát triển với live reload cho Gateway, auth-service, user-service, book-service và worker-service:

```bash
make dev
make dev-logs
```

Air theo dõi source được mount vào container và tự build/restart process khi file `.go`, `.toml`, `.yaml` hoặc `.yml` thay đổi. Dùng `make dev-down` để dừng development stack nhưng giữ nguyên Docker volumes.

Các địa chỉ:

- API Gateway: `http://localhost:8080`
- Swagger UI: `http://localhost:8080/swagger/index.html`
- PostgreSQL: `localhost:5432`
- pgAdmin: `http://localhost:5050`
- Redis: `localhost:6379`
- RedisInsight: `http://localhost:5540`
- RabbitMQ AMQP: `localhost:5672`
- RabbitMQ Management: `http://localhost:15672`

Đăng nhập pgAdmin:

```text
Email:    admin@bookstore.com
Password: admin
```

Khi tạo server trong pgAdmin, dùng host `postgres`, port `5432`, database/user/password đều là `bookstore`.

Trong RedisInsight, tạo connection tới host `redis`, port `6379`.

Đăng nhập RabbitMQ Management bằng user/password `bookstore`. Exchange `bookstore.events` là durable topic exchange; queue `user.profile.create` và dead-letter queue `user.profile.create.dead` đều là quorum queue.

## Thử API

Kiểm tra Gateway:

```bash
curl http://localhost:8080/healthz
```

Tạo tài khoản:

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

Đọc profile:

```bash
curl http://localhost:8080/api/v1/users/me \
  -H 'Authorization: Bearer YOUR_ACCESS_TOKEN'
```

Profile được tạo bất đồng bộ. Ngay sau register có thể có một khoảng trễ rất ngắn trước khi `/users/me` trả dữ liệu; frontend nên retry khi nhận `404` trong giai đoạn này.

Danh sách sách là public:

```bash
curl 'http://localhost:8080/api/v1/books?page=1&page_size=20'
```

Endpoint ghi sách yêu cầu role `admin`. Để gán role khi phát triển local:

```bash
docker compose exec -T postgres psql -U bookstore -d bookstore \
  -c "UPDATE auth.accounts SET roles = ARRAY['customer','admin'] WHERE email = 'reader@example.com'"
```

Sau đó đăng nhập lại để nhận JWT mới.

## Lệnh phát triển

```bash
make help
make check
make generate
make proto
make swagger
make fmt
make test
make vet
make build
make up
make ps
make logs
make down
make dev
make dev-logs
make dev-down
```

`make proto` cài Buf và protobuf plugins vào `backend/.tools`, không cài `protoc` toàn hệ thống.
`make swagger` sinh lại `docs/docs.go`, `docs/swagger.json` và `docs/swagger.yaml` từ annotations trên HTTP handler. Swagger sử dụng DTO do Gateway tự định nghĩa, không expose model generated từ protobuf.

## API hiện tại

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `GET /api/v1/users/me`
- `PUT /api/v1/users/me`
- `GET /api/v1/books`
- `GET /api/v1/books/:id`
- `POST /api/v1/admin/books`
- `PUT /api/v1/admin/books/:id`
- `DELETE /api/v1/admin/books/:id`

## Ghi chú production

- Transactional outbox đã dùng PostgreSQL + RabbitMQ publisher confirms. Production nên chạy RabbitMQ cluster ba node, áp policy at-least-once cho DLX, thêm metrics/alert cho pending event và archive bảng outbox.
- JWT demo dùng HMAC. Khi chuyển Keycloak/Auth0, thay token adapter bằng OIDC/JWKS; application và domain không cần đổi.
- gRPC đang dùng plaintext trong private Docker network. Production nên bật mTLS/service identity.
- `docker-entrypoint-initdb.d` chỉ phù hợp bootstrap local. CI/CD production nên chạy migration job có version riêng.
