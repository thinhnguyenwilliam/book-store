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
  account_deleted_routing_key: "account.deleted"
```

Auth local dùng access token JWT `5m` và refresh session `168h`. Gateway đặt refresh token vào cookie `HttpOnly`, `SameSite=Lax`, còn PostgreSQL chỉ lưu SHA-256 hash của token:

```yaml
gateway:
  allowed_origins: ["http://localhost:5173"]
  refresh_cookie_name: "bookstore_refresh"
  refresh_cookie_secure: false # production bắt buộc true với HTTPS
  refresh_cookie_same_site: "lax"
  request_timeout: "2s"        # deadline tổng để hủy DB/gRPC khi request bị treo
  performance_target: "200ms"  # SLO quan sát, không phải hard timeout
  read_header_timeout: "2s"
  read_timeout: "5s"
  write_timeout: "10s"
  idle_timeout: "60s"

postgres:
  max_open_connections: 25
  max_idle_connections: 10
  connection_max_lifetime: "30m"
  connection_max_idle_time: "5m"

auth:
  access_token_ttl: "5m"
  refresh_token_ttl: "168h"
  google_client_id: "your-web-client-id.apps.googleusercontent.com"
```

`google_client_id` là OAuth Web Client ID công khai, không phải client secret. Endpoint `POST /api/v1/auth/google` nhận Google ID token trong trường `credential`; Auth Service kiểm tra chữ ký, audience, issuer, expiration, `email_verified` và lưu Google `sub` làm identity ổn định. Migration `007_account_identities.sql` tạo bảng liên kết identity với account hiện có.

Compose mount file này read-only vào container và truyền đường dẫn bằng CLI flag:

```text
/app/service -config /app/config.yml
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

## Structured log và xoay file hằng ngày

Khi chạy Go/Air local, mỗi process ghi structured log ra terminal và một file riêng trong `backend/logs`:

```text
logs/gateway-2026-08-20.log
logs/authservice-2026-08-20.log
logs/userservice-2026-08-20.log
logs/bookservice-2026-08-20.log
logs/workerservice-2026-08-20.log
```

Một rotation worker chạy trong từng process và mở file mới đúng `00:00` theo `logging.timezone`. Writer cũng kiểm tra lại ngày ở mỗi lần ghi, nên vẫn đổi đúng file nếu máy sleep hoặc timer bị trễ. Khi shutdown, worker được dừng và file được đóng. Thư mục log đã nằm trong `.gitignore`.

File đang ghi của mỗi ngày luôn giữ tên `service-YYYY-MM-DD.log`. Nếu file đạt `logging.max_size_mb`, writer lưu phần đầy thành `service-YYYY-MM-DD.001.log`, `.002.log`... rồi tiếp tục ghi vào file chính mới. Cleanup chỉ quản lý file có đúng prefix của service hiện tại:

- `max_age_days`: xoá backup cũ hơn số ngày cấu hình; `0` để không giới hạn theo tuổi.
- `max_backups`: chỉ giữ số backup mới nhất; `0` để không giới hạn số lượng.
- File đang được ghi không bao giờ bị cleanup xoá.

Khi chạy Docker, file nằm ở `/app/logs` và được giữ trong named volume `app-logs`; `also_stdout` vẫn cho phép xem bằng `make logs` và chuyển log sang Loki/ELK/OpenSearch ở production.

Local dùng text với thời gian dễ đọc như `20/08/2026 16:10:30.189 +07:00`; cấu hình Docker dùng JSON với timestamp RFC3339 để hệ thống thu thập log dễ parse:

```yaml
logging:
  directory: "logs"
  level: "info"
  format: "json"
  timezone: "Asia/Ho_Chi_Minh"
  also_stdout: true
  max_size_mb: 100
  max_age_days: 14
  max_backups: 30
```

Log HTTP chứa request ID, trace ID, method, URI, status, latency và response size. Password, JWT, refresh token và request body không được ghi log.

### Trace ID xuyên microservice

Gateway nhận header `X-Trace-ID` gồm 32 ký tự hex hoặc tự sinh một ID mới, rồi trả ID đó trong response header. ID được đặt vào context để structured logger tự thêm trường `trace_id` và được truyền qua:

```text
HTTP Gateway
  -> gRPC metadata -> Auth/User/Book
  -> outbox trace_id -> RabbitMQ header
  -> Worker -> gRPC metadata -> User
```

`request_id` nhận diện một HTTP request cụ thể; `trace_id` là correlation ID được giữ xuyên các service và cả đoạn xử lý bất đồng bộ. Có thể gửi ID cố định lúc debug:

```bash
curl -i http://localhost:8080/api/v1/books?limit=2 \
  -H 'X-Trace-ID: 0123456789abcdef0123456789abcdef'

rg '0123456789abcdef0123456789abcdef' logs/
```

Request register mẫu trong `api.http` dùng trace ID dễ nhận biết. Outbox lưu ID trong cột `auth.outbox_events.trace_id`, vì vậy trace không bị mất nếu RabbitMQ down hoặc event được retry sau khi HTTP request đã kết thúc.

## Các kiểu gRPC trong backend

gRPC hỗ trợ bốn kiểu RPC:

- Unary: client gửi một request, server trả một response.
- Server streaming: client gửi một request, server trả một luồng nhiều response.
- Client streaming: client gửi một luồng nhiều request, server trả một response.
- Bidirectional streaming: client và server đồng thời gửi nhiều message trên cùng connection.

Các contract Auth, User và Book hiện đều là unary vì CRUD, login và cursor pagination là mô hình request/response thông thường. Streaming chỉ nên thêm khi nghiệp vụ cần, ví dụ live inventory, import sách theo batch hoặc đồng bộ sự kiện hai chiều. Server đã đăng ký cả unary và stream interceptor; Gateway/Worker cũng đăng ký cả unary và stream client interceptor, nên RPC streaming thêm sau này vẫn được log và recover panic mà không phải thay hạ tầng chung.

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

Xoá khách hàng dùng cùng nguyên tắc theo chiều ngược lại:

```text
DELETE /api/v1/admin/customers/:id
  -> Auth Service: DELETE auth.accounts + INSERT account.deleted (cùng transaction)
  -> 202 Accepted
  -> outbox -> RabbitMQ -> Worker -> User Service: DELETE users.user_profiles
```

Refresh sessions bị PostgreSQL xoá cascade cùng account. `VerifyToken` kiểm tra account còn tồn tại nên access token cũ mất hiệu lực ngay. Consumer xoá profile theo cách idempotent; event được giao lặp lại vẫn thành công. Gateway không cho admin tự xoá chính tài khoản đang đăng nhập.

## Graceful shutdown

Gateway, các gRPC service, outbox dispatcher và RabbitMQ worker xử lý `SIGINT`/`SIGTERM` theo grace period cấu hình tại `shutdown.timeout`. Giá trị local mặc định là `12s`; Docker và Air chờ `15s` trước khi force kill để process có thời gian:

- ngừng nhận HTTP/gRPC request và RabbitMQ delivery mới;
- hoàn tất request, publisher confirm và message handler đang chạy;
- đóng RabbitMQ channel/connection, gRPC connection và PostgreSQL pool theo đúng thứ tự.

Nếu worker không drain xong trước deadline, service trả lỗi shutdown thay vì chờ vô hạn. Message chưa ACK sẽ được RabbitMQ đưa lại queue.

## Chạy toàn bộ bằng Docker

```bash
cd ~/WorkSpace/Book-store/backend
make up
```

Lệnh trên tương đương `docker compose up -d --build`. Dùng `make logs` để theo dõi log và `make ps` để xem trạng thái container.

## Chạy Go service trực tiếp trên máy

Chuẩn bị infrastructure nhưng không build/chạy container Go:

```bash
make local-prepare
```

Lệnh này dừng `gateway`, `auth-service`, `user-service`, `book-service`, `worker-service` trong Docker và chỉ giữ PostgreSQL, pgAdmin, Redis, RedisInsight, RabbitMQ. Các Docker volume dữ liệu không bị xóa.

Mở năm terminal trong thư mục `backend`:

```bash
# Terminal 1
make local-auth

# Terminal 2
make local-user

# Terminal 3
make local-book

# Terminal 4
make local-worker

# Terminal 5
make local-gateway
```

Các lệnh trên dùng `go run` và [config/local.yml](config/local.yml). Nếu muốn Air hot reload trực tiếp trên máy, dùng tương ứng:

```bash
make watch-auth
make watch-user
make watch-book
make watch-worker
make watch-gateway
```

Air được cài vào `.tools/air` ở lần chạy `watch-*` đầu tiên, không cài global và không build Docker image.

### Air hot reload bên trong Docker

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
curl -c /tmp/bookstore-cookies.txt -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"reader@example.com","password":"password123"}'
```

Làm mới access token và rotate refresh token:

```bash
curl -b /tmp/bookstore-cookies.txt -c /tmp/bookstore-cookies.txt \
  -X POST http://localhost:8080/api/v1/auth/refresh
```

Đăng xuất và revoke refresh session:

```bash
curl -b /tmp/bookstore-cookies.txt -X POST http://localhost:8080/api/v1/auth/logout
```

Đọc profile:

```bash
curl http://localhost:8080/api/v1/users/me \
  -H 'Authorization: Bearer YOUR_ACCESS_TOKEN'
```

Profile được tạo bất đồng bộ. Ngay sau register có thể có một khoảng trễ rất ngắn trước khi `/users/me` trả dữ liệu; frontend nên retry khi nhận `404` trong giai đoạn này.

Danh sách sách là public:

```bash
curl 'http://localhost:8080/api/v1/books?limit=20'
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
make lint
make test
make vet
make build
make perf
make up
make ps
make logs
make down
make dev
make dev-logs
make dev-down
make local-prepare
make infra-up
make infra-ps
make infra-logs
make infra-stop
make app-stop
make local-auth
make local-user
make local-book
make local-worker
make local-gateway
make watch-auth
make watch-user
make watch-book
make watch-worker
make watch-gateway
```

`make proto` cài Buf và protobuf plugins vào `backend/.tools`, không cài `protoc` toàn hệ thống.
`make swagger` sinh lại `docs/docs.go`, `docs/swagger.json` và `docs/swagger.yaml` từ annotations trên HTTP handler. Swagger sử dụng DTO do Gateway tự định nghĩa, không expose model generated từ protobuf.

## Linter và mục tiêu API dưới 200 ms

`make lint` dùng golangci-lint v2 đã pin version trong Makefile và tự cài binary vào `backend/.tools`; không phụ thuộc bản cài global. `make check` chạy lần lượt lint, unit test, `go vet`, build và kiểm tra Docker Compose.

Gateway ghi `duration_ms`, `slo_target_ms` và `slo_met` cho **mọi HTTP request**. Request từ `200ms` trở lên được log ở mức `WARN`. Mục tiêu 200 ms là SLO p95; hard timeout tổng đang là 2 giây để request chậm có thời gian trả lỗi có kiểm soát và hủy tiếp các lệnh gRPC/PostgreSQL.

Khi bốn service API local đang chạy, đo các read endpoint an toàn bằng 100 request, concurrency 10:

```bash
make perf
```

Đo thêm API cần đăng nhập và quyền admin:

```bash
PERF_ACCESS_TOKEN='YOUR_ACCESS_TOKEN' make perf
```

Có thể truyền thêm `PERF_BOOK_ID`, `PERF_REQUESTS`, `PERF_CONCURRENCY` và `PERF_P95_TARGET_MS`. Performance gate cố ý không gọi hàng loạt endpoint ghi/xóa để tránh làm biến đổi dữ liệu local. Kết quả trên laptop không bảo đảm production luôn dưới 200 ms; production vẫn cần theo dõi p95/p99 theo route dưới tải thật và đặt alert cho `slo_met=false`.

Phần truy cập PostgreSQL bật prepared-statement cache, bỏ default transaction của GORM cho các câu lệnh đơn và cấu hình connection pool. Những workflow cần tính nguyên tử như tạo account + refresh session + outbox vẫn dùng transaction tường minh, nên không bị mất tính nhất quán.

## API hiện tại

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/logout`
- `GET /api/v1/users/me`
- `PUT /api/v1/users/me`
- `GET /api/v1/books?limit=20&cursor=<opaque-cursor>`
- `GET /api/v1/books/:id`
- `POST /api/v1/admin/books`
- `PUT /api/v1/admin/books/:id`
- `DELETE /api/v1/admin/books/:id`

## Ghi chú production

- Transactional outbox đã dùng PostgreSQL + RabbitMQ publisher confirms. Production nên chạy RabbitMQ cluster ba node, áp policy at-least-once cho DLX, thêm metrics/alert cho pending event và archive bảng outbox.
- Access token JWT dùng HMAC và sống `5m`; refresh token opaque sống `168h`, được rotate mỗi lần dùng, revoke theo session family khi phát hiện token cũ bị dùng lại và chỉ lưu hash trong `auth.refresh_sessions`. Production phải bật HTTPS, đặt `refresh_cookie_secure: true`, dùng secret manager cho JWT secret và chỉ whitelist origin storefront thật.
- Khi chuyển Keycloak/Auth0, thay token adapter bằng OIDC/JWKS; application và domain không cần đổi.
- gRPC đang dùng plaintext trong private Docker network. Production nên bật mTLS/service identity.
- `docker-entrypoint-initdb.d` chỉ phù hợp bootstrap local. CI/CD production nên chạy migration job có version riêng.
