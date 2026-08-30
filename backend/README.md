# Book Store Backend

Backend gồm mười process Go độc lập và một nhóm hạ tầng dùng cho local development:

```text
Storefront / Admin Portal
           |
           | HTTP/JSON :8080
           v
      Echo API Gateway
        /     |      \
        gRPC service mesh
   /      /      |       \        \
 Auth   User    Book    Order    Payment    Notification    Comment    Chat
  |         ^           |
  | outbox  | gRPC      |
  v         |           |
RabbitMQ -> Worker       |
   \         |         /
    PostgreSQL / bookstore
      |       |       |
 auth  users  catalog  orders  payments  notifications  comments  chat  <- schemas

RabbitMQ xử lý domain event; Redis được giữ riêng cho cache/rate-limit.
```

Gateway là public entry point. Các service giao tiếp bằng unary gRPC; Order Service điều phối Book Service và Payment Service cho checkout Saga.

## Một database hay ba database?

Local hiện chỉ chạy **một PostgreSQL container và một database `bookstore`**. Database này có tám schema:

- `auth`: do auth-service sở hữu.
- `users`: do user-service sở hữu.
- `catalog`: do book-service sở hữu.
- `orders`: do order-service sở hữu.
- `payments`: do payment-service sở hữu.
- `notifications`: do notification-service sở hữu; gồm inbox event, thông báo trong app và trạng thái gửi email.
- `comments`: do comment-service sở hữu; chứa thread bình luận và câu trả lời.
- `chat`: do chat-service sở hữu; chứa support conversation, member, message, read cursor và transactional outbox.

“Service sở hữu dữ liệu” nghĩa là service khác không query hoặc sửa trực tiếp schema đó; nó phải gọi gRPC/API của service sở hữu. Điều này không bắt buộc mỗi service phải có một PostgreSQL server riêng. Khi hệ thống lớn hơn, từng schema có thể được chuyển sang database/server riêng mà không đổi domain và use case.

Foreign key chỉ được khai báo giữa các bảng do cùng một service sở hữu. Migration `011_intraservice_foreign_keys.sql` bảo vệ Order aggregate, stock reservation và các bảng financial ledger/payment. Các ID xuyên service như `user_id`, `book_id` trong Order hoặc `order_id` trong Payment vẫn là logical reference và được kiểm tra qua gRPC/event, không tạo foreign key xuyên bounded context.

## Cấu hình Viper

Tất cả Go process đọc [config/config.yml](config/config.yml) bằng Viper. Không dùng `.env`:

```yaml
postgres:
  url: "postgres://bookstore:bookstore@postgres:5432/bookstore?sslmode=disable"

redis:
  enabled: true
  address: "redis:6379"
  password: ""
  database: 0
  namespace: "bookstore"
  dial_timeout: "500ms"
  read_timeout: "50ms"
  write_timeout: "50ms"
  pool_size: 20
  book_ttl: "1m"
  cart_ttl: "5m"
  lock_ttl: "3s"

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

grpc:
  call_timeout: "1500ms"       # timeout tối đa cho mỗi unary RPC

postgres:
  max_open_connections: 25
  max_idle_connections: 10
  connection_max_lifetime: "30m"
  connection_max_idle_time: "5m"

auth:
  access_token_ttl: "5m"
  refresh_token_ttl: "168h"
  google_client_id: "your-web-client-id.apps.googleusercontent.com"
  facebook_app_id: "your-meta-app-id"
  facebook_app_secret: "server-only-meta-app-secret"
  facebook_graph_version: "v25.0"

payment:
  currency: "VND"
  platform_owner_id: "platform"
  funding_owner_id: "system:funding"
  clearing_owner_id: "gateway:vnpay:clearing"
  default_provider: "wallet" # đổi thành vnpay sau khi cấu hình credential
  platform_fee_bps: 1000 # 10%, đơn vị basis point
  reconcile_interval: "1m"
  reconcile_grace: "2m"
  reconcile_batch_size: 100
  vnpay:
    enabled: false
    pay_url: "https://sandbox.vnpayment.vn/paymentv2/vpcpay.html"
    api_url: "https://sandbox.vnpayment.vn/merchant_webapi/api/transaction"
    tmn_code: "YOUR_VNPAY_TMN_CODE"
    hash_secret: "YOUR_SERVER_ONLY_HASH_SECRET"
    return_url: "http://localhost:5173/thanh-toan/ket-qua"
    server_ip: "YOUR_PUBLIC_SERVER_IP"
    timezone: "Asia/Ho_Chi_Minh"
    expire_after: "15m"
    http_timeout: "5s"
```

`tmn_code` và `hash_secret` chỉ nằm trong file YAML local bị ignore hoặc secret manager/mounted secret ở production; không đưa vào frontend hay commit Git. `return_url` là trang browser quay về, không phải nguồn xác nhận thanh toán. Backend chỉ chuyển `pending` sang `succeeded` từ IPN đã xác thực hoặc kết quả query đối soát.

`google_client_id` là OAuth Web Client ID công khai, không phải client secret. Endpoint `POST /api/v1/auth/google` nhận Google ID token trong trường `credential`; Auth Service kiểm tra chữ ký, audience, issuer, expiration, `email_verified` và lưu Google `sub` làm identity ổn định. Migration `007_account_identities.sql` tạo bảng liên kết identity với account hiện có.

Endpoint `POST /api/v1/auth/facebook` nhận Facebook user access token. Auth Service dùng App ID + App Secret gọi `debug_token`, sau đó lấy `id,name,email` bằng Graph API và gửi `appsecret_proof`. Facebook App Secret chỉ được đặt trong backend. Khi chạy local, copy `config/local.yml.example` thành `config/local.yml`; file thật đã được `.gitignore` bỏ qua và không còn được Git theo dõi.

### Bảo mật Google/Facebook login

Storefront và admin portal đang dùng popup callback của Google GIS và Facebook JavaScript SDK, không dùng authorization-code redirect callback. Vì vậy runtime không tự ghép `redirect_uri`; authorized JavaScript origins, Facebook Site URL và Valid OAuth Redirect URIs phải cấu hình chính xác trong console của provider, chỉ dùng HTTPS production và không dùng wildcard hoặc giá trị nhận từ query string.

Trước khi mở provider, frontend gọi `POST /api/v1/auth/provider-state`. Gateway sinh 256-bit random state, trả state trong JSON và đặt bản đối chiếu trong cookie `HttpOnly`, `SameSite`, sống 10 phút. State được bind theo provider và `create_account`; Google còn nhận cùng giá trị qua cả button `state` và ID-token `nonce`. Gateway so sánh cookie bằng constant-time comparison, còn Auth Service bắt buộc nonce trong ID token phải khớp. Cookie state được xóa sau đăng nhập thành công.

Google ID token và Facebook user access token chỉ được dùng một lần để xác minh danh tính rồi bỏ, không lưu database và không log. Project không xin quyền gọi Google/Facebook API sau đăng nhập nên không cần provider refresh token. Session của Book Store dùng access token JWT `5m` và opaque refresh token trong cookie HttpOnly `168h`; refresh token được rotate/reuse-detect như luồng đăng nhập mật khẩu.

Lỗi provider có envelope ổn định với `error.code`, `error.provider` và `error.retryable`. Các code chính gồm `invalid_oauth_state`, `invalid_provider_credential`, `external_identity_conflict`, `provider_not_configured`, `provider_timeout`, `provider_unavailable` và `external_login_failed`; raw response từ Google/Meta không được trả thẳng cho browser.

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
internal/gateway/graphql/        schema, resolvers và runtime GraphQL sinh bởi gqlgen
migrations/                      schema của database bookstore
```

Các PostgreSQL adapter sử dụng GORM. Domain không import Echo, gRPC, GORM, Viper hoặc JWT.

### Mapping HTTP/JSON và Protobuf

Gateway không trả generated protobuf trực tiếp cho frontend. DTO public nằm trong
`internal/gateway/http/dto.go`; toàn bộ chuyển đổi JSON DTO ↔ protobuf được gom tại
`internal/gateway/http/mapper.go`. Ở phía service, delivery gRPC tiếp tục map
protobuf ↔ domain model, nên domain và application layer không phụ thuộc contract
transport sinh bởi `protoc`.

Lỗi domain được service đổi thành gRPC status, sau đó Gateway đổi sang HTTP theo
một bảng thống nhất: `InvalidArgument → 400`, `Unauthenticated → 401`,
`PermissionDenied → 403`, `NotFound → 404`, `AlreadyExists/Aborted → 409`,
`ResourceExhausted → 429`, `DeadlineExceeded → 504`, `Unimplemented → 501` và
`Unavailable → 503`. Lỗi nội bộ không được trả nguyên văn ra client.

### Context deadline

`gateway.request_timeout` là ngân sách tổng của HTTP request. Context này mang
cancellation, deadline, request ID và trace ID xuống gRPC. `grpc.call_timeout` là
giới hạn riêng cho mỗi unary RPC; interceptor chỉ thêm giới hạn này khi caller
chưa có deadline sớm hơn. Vì vậy khi browser hủy request hoặc hết deadline, DB,
external identity provider và service downstream đều nhận cùng tín hiệu hủy.

### GraphQL aggregation API

`POST /graphql` là lớp query bổ sung tại Gateway; REST và gRPC hiện tại không bị thay thế. Ba root query đầu tiên:

- `bookDetail`: public, gọi Book Service và Comment Service song song.
- `adminDashboard`: yêu cầu role `admin`, gọi Book Service và User Service song song. Các chỉ số catalog/customer chỉ phản ánh cursor page được yêu cầu, không giả làm global total.
- `orderDetail`: yêu cầu đăng nhập, Order Service kiểm tra ownership trước khi Gateway ghép Payment Service.

Gateway giới hạn body `64K`, parser token, query depth và complexity. Introspection bật trong `local.yml.example` để phát triển nhưng tắt trong `config.yml` mặc định production. GraphQL chỉ phục vụ query qua POST; các command đăng nhập, giỏ hàng, tạo đơn, thanh toán và webhook tiếp tục dùng REST.

Sau khi sửa `schema.graphqls`, chạy:

```bash
make graphql
```

Pre-commit hook và CI cùng sinh lại mã gqlgen rồi kiểm tra `git diff`, vì vậy generated runtime/model không thể lệch schema.

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

Lệnh này dừng toàn bộ mười container Go rồi chỉ giữ PostgreSQL, pgAdmin, Redis, RedisInsight, RabbitMQ và Mailpit. Các Docker volume dữ liệu không bị xóa.

Mở mười terminal trong thư mục `backend`:

```bash
# Terminal 1
make local-auth

# Terminal 2
make local-user

# Terminal 3
make local-book

# Terminal 4
make local-payment

# Terminal 5
make local-order

# Terminal 6
make local-worker

# Terminal 7
make local-notification

# Terminal 8
make local-comment

# Terminal 9
make local-chat

# Terminal 10
make local-gateway
```

Các lệnh trên dùng `go run` và [config/local.yml](config/local.yml). Nếu muốn Air hot reload trực tiếp trên máy, dùng tương ứng:

```bash
make watch-auth
make watch-user
make watch-book
make watch-payment
make watch-order
make watch-worker
make watch-notification
make watch-comment
make watch-chat
make watch-gateway
```

Air được cài vào `.tools/air` ở lần chạy `watch-*` đầu tiên, không cài global và không build Docker image.

### Air hot reload bên trong Docker

Để phát triển với live reload cho toàn bộ mười Go process:

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
- Mailpit inbox: `http://localhost:8025`

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
make graphql
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
make local-order
make local-payment
make local-worker
make local-notification
make local-gateway
make watch-auth
make watch-user
make watch-book
make watch-order
make watch-payment
make watch-worker
make watch-notification
make watch-gateway
```

`make proto` cài Buf và protobuf plugins vào `backend/.tools`, không cài `protoc` toàn hệ thống.
`make swagger` sinh lại `docs/docs.go`, `docs/swagger.json` và `docs/swagger.yaml` từ annotations trên HTTP handler. Swagger sử dụng DTO do Gateway tự định nghĩa, không expose model generated từ protobuf.
`make graphql` sinh runtime/model type-safe từ `internal/gateway/graphql/schema.graphqls` bằng phiên bản gqlgen được pin trong `go.mod`.

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
- `GET /api/v1/cart/items`
- `POST /api/v1/cart/items`
- `PUT /api/v1/cart/items/:id`
- `DELETE /api/v1/cart/items/:id`
- `DELETE /api/v1/cart/items`
- `POST /api/v1/cart/items/batch-delete`
- `POST /api/v1/orders`
- `GET /api/v1/orders`
- `GET /api/v1/orders/:id`
- `PUT /api/v1/orders/:id/cancel`
- `POST /api/v1/payments`
- `GET /api/v1/payments/webhooks/vnpay` (public IPN, bắt buộc chữ ký VNPAY)
- `GET /api/v1/payments/:id`
- `GET /api/v1/payments/order/:order_id`
- `POST /api/v1/wallets/me`
- `GET /api/v1/wallets/me`
- `PUT /api/v1/admin/wallets/:owner_id/balance`
- `GET /api/v1/notifications?limit=20&cursor=<opaque-cursor>`
- `GET /api/v1/notifications/unread-count`
- `PUT /api/v1/notifications/:id/read`
- `PUT /api/v1/notifications/read-all`
- `POST /api/v1/notifications/devices`
- `DELETE /api/v1/notifications/devices/:device_id`
- `GET /api/v1/books/:id/comments?limit=20&cursor=<opaque-cursor>`
- `POST /api/v1/books/:id/comments`
- `GET /api/v1/comments/:id/replies?limit=50&cursor=<opaque-cursor>`
- `PUT /api/v1/comments/:id`
- `DELETE /api/v1/comments/:id`
- `PUT /api/v1/admin/comments/:id/status`
- `POST /api/v1/chat/conversations/support`
- `GET /api/v1/chat/conversations?limit=20&cursor=<opaque-cursor>`
- `GET /api/v1/chat/conversations/:id/messages?limit=30&cursor=<opaque-cursor>`
- `POST /api/v1/chat/conversations/:id/messages`
- `PUT /api/v1/chat/conversations/:id/read`
- `GET /api/v1/chat/unread-count`
- `PUT /api/v1/chat/messages/:id`
- `DELETE /api/v1/chat/messages/:id`
- `POST /api/v1/chat/ws-ticket`
- `GET /api/v1/chat/ws?ticket=<one-time-ticket>` (WebSocket upgrade)

### Cây comment và reply

Comment Service dùng adjacency list mở rộng thay vì Nested Set. `parent_id` trỏ tới comment cha trực tiếp, `root_id` gom toàn bộ thread và `depth` giới hạn tối đa ba cấp. Cách này cho phép insert reply theo O(1), tránh phải cập nhật hàng loạt chỉ số trái/phải và giảm lock khi nhiều người bình luận đồng thời.

Comment gốc được phân trang cursor theo thời gian giảm dần; replies của từng thread dùng cursor theo thời gian tăng dần. Xoá comment là soft delete để giữ cấu trúc khi comment cha đã có câu trả lời. `parent_id` và `root_id` có self foreign key thật vì đều nằm trong Comment Service; `book_id` và `author_id` là logical reference xuyên service, được xác minh qua Book/User gRPC khi tạo comment.

### Chat hỗ trợ realtime

Chat Service lưu conversation, member, message và read cursor trong PostgreSQL. Mỗi khách hàng có tối đa một support conversation trạng thái `open`; admin được xem tất cả support conversation và được thêm làm member khi phản hồi. `sequence_number` được tăng dưới row lock của conversation để bảo đảm thứ tự, còn unique `(sender_id, client_message_id)` làm retry idempotent.

Browser không đặt access token hoặc refresh token vào WebSocket URL. Frontend gọi `POST /api/v1/chat/ws-ticket` bằng bearer access token để lấy ticket ngẫu nhiên 256-bit, sống 30 giây và chỉ dùng một lần; Gateway dùng `GETDEL` trên Redis trước khi upgrade. Origin của handshake phải nằm trong `gateway.allowed_origins`.

PostgreSQL là nguồn dữ liệu chính. Redis giữ ticket, presence có TTL và Pub/Sub giữa nhiều Gateway replica; nếu frame realtime bị bỏ lỡ, client reconnect rồi đồng bộ lại bằng cursor API. Transactional outbox `chat.outbox_events` phát `chat.message.created` qua RabbitMQ để Notification Service tạo thông báo trong app. Chat tần suất cao nên phase này không gửi một email cho từng message.

WebSocket nhận các command `message.send`, `conversation.read`, `typing.changed`, `ping` và phát các event `message.created`, `message.updated`, `message.deleted`, `conversation.read`, `typing.changed`, `presence.changed`, `pong`, `error`. Tin nhắn tối đa 4.000 ký tự; sender lấy từ ticket đã xác thực, không tin `sender_id` do browser gửi.

## Checkout Saga, stock và ledger

`POST /api/v1/orders` snapshot title/price/seller từ Book Service rồi tạo stock reservation. Stock khả dụng được giảm trong transaction của Book Service, nhưng reservation vẫn có trạng thái riêng để `CommitStock` hoặc `ReleaseStock` chạy idempotent. Không gọi `DecreaseStock` mù.

`POST /api/v1/payments` yêu cầu header `Idempotency-Key`. Với `provider=wallet`, Payment Service ghi debit buyer, credit seller và credit platform fee trong **một transaction PostgreSQL**; tổng các ledger entry luôn bằng zero. Với `provider=vnpay`, API trả payment `pending` cùng `checkout_url`; frontend redirect browser tới URL đó. `UpdateBalance` cũng tạo cặp ledger entry với funding wallet, không overwrite balance. Endpoint balance chỉ nằm dưới `/admin`.

### Thanh toán tiền thật bằng VNPAY

```text
POST /payments (provider=vnpay)
  -> lưu payment pending + allocations
  -> trả checkout_url -> browser sang VNPAY

VNPAY IPN -> GET /payments/webhooks/vnpay
  -> verify HMAC-SHA512 trước khi dùng payload
  -> kiểm tra provider reference + amount + trạng thái
  -> cùng một PostgreSQL transaction:
       ghi webhook inbox (chống xử lý trùng)
       post double-entry ledger
       cập nhật payment
       ghi payments.outbox_events
  -> dispatcher publish-confirm -> RabbitMQ -> Order Service
```

Payment Service chạy settlement reconciler theo `payment.reconcile_interval`. Các payment `pending`/`refund_pending` quá `reconcile_grace` được gọi VNPAY `querydr`; kết quả và mismatch được lưu vào `payments.settlement_reconciliations`. Mismatch reference/amount chỉ được ghi nhận để điều tra, không tự sửa ledger.

Hoàn tiền VNPAY dùng API `refund` có checksum và request ID xác định từ idempotency key. Ledger nội bộ chỉ đảo khi provider xác nhận `refunded`; phản hồi đang xử lý được lưu `refund_pending` và reconciler theo dõi tiếp. Việc đảo ledger và tạo event `payment.refunded` nằm trong cùng transaction. Không dùng Return URL của browser để cộng tiền, trừ tiền hoặc xác nhận order.

Thiết lập sandbox:

1. Điền `payment.vnpay.tmn_code` và `payment.vnpay.hash_secret` trong `config/local.yml` (file này đã ignore).
2. Đặt `payment.vnpay.enabled: true`; giữ `default_provider: wallet` nếu muốn chọn provider theo từng request, hoặc đổi thành `vnpay`.
3. Khai báo IPN URL trên VNPAY là `https://YOUR_API_HOST/api/v1/payments/webhooks/vnpay`. Localhost cần HTTPS tunnel để VNPAY gọi được.
4. Chạy `make migrate`, sau đó khởi động Payment, Order và Gateway cùng PostgreSQL/RabbitMQ.

Order Service là Saga orchestrator:

```text
pending -> stock_reserved -> payment_pending -> confirmed
              |                 |
              |                 +-> payment declined -> release stock -> cancelled
              |                 +-> commit error -> refund ledger + release stock -> cancelled
              |                 +-> compensation error -> compensation_pending
              +-> reservation expired -> release stock -> cancelled
```

Client phải giữ nguyên `Idempotency-Key` khi retry cùng thao tác. Payment bị timeout được tra lại bằng `order_id` trước khi compensation, tránh trường hợp thanh toán đã thành công nhưng response bị mất. Order Service có reconciler chạy nền: hoàn tất payment bị mất response, hủy reservation hết hạn và retry trạng thái `compensation_pending`. Trong microservice không có rollback ACID xuyên database; refund và release stock là các transaction bù trừ có lịch sử riêng.

Để chạy flow local, dùng tuần tự các request trong [api.http](api.http): tạo wallet, admin fund wallet, thêm sách vào cart, tạo order rồi thanh toán. Book có `seller_id`; nếu bỏ trống thì doanh thu được gán cho platform wallet.

## Notification Service phase 1

Notification Service subscribe `account.registered`, `payment.succeeded`, `payment.failed` và `payment.refunded` từ RabbitMQ. Mỗi `MessageId` được ghi vào `notifications.inbox_events` trong cùng transaction với thông báo và email delivery, nên RabbitMQ redelivery không tạo bản ghi trùng.

Thông báo trong app được đọc qua Gateway bằng access token; Gateway luôn lấy `user_id` từ principal, không nhận user ID tùy ý từ browser. Storefront hiển thị chuông, số chưa đọc, danh sách gần nhất và tự refresh mỗi 30 giây.

Email local gửi qua SMTP Mailpit tại `localhost:1025`; xem thư ở `http://localhost:8025`. Có thể tắt bằng `notification.email_enabled: false`. Email worker claim delivery bằng `FOR UPDATE SKIP LOCKED`, retry tách khỏi RabbitMQ theo `email_retry_delay` và dừng tại `email_max_attempts`; vì vậy SMTP outage không chặn domain event và nhiều replica không cùng claim một delivery. Production phải đặt SMTP credential trong secret YAML/secret manager, bật STARTTLS và không commit password. SMTP có bản chất at-least-once: trường hợp server đã nhận thư nhưng connection mất trước response cuối có thể phát sinh một email trùng; provider hỗ trợ idempotency key sẽ xử lý tốt hơn SMTP thuần.

### Firebase Cloud Messaging

FCM là delivery channel bổ sung cho Web Push, không thay PostgreSQL, RabbitMQ, email hoặc WebSocket. Domain event vẫn được ACK sau khi inbox/notification/push delivery commit; push worker claim `notifications.push_deliveries` bằng `FOR UPDATE SKIP LOCKED`, retry có delay và giới hạn số lần. FCM trả `UNREGISTERED` thì installation bị vô hiệu hóa và các delivery còn chờ của installation đó được đánh dấu `skipped`.

FCM mặc định tắt để project vẫn chạy khi chưa có Firebase account. Để bật local:

1. Tạo Firebase project và hai Web App (storefront/admin có thể dùng cùng project), bật Cloud Messaging và tạo Web Push VAPID key.
2. Copy `.env.example` thành `.env` ở mỗi frontend rồi điền các biến `VITE_FIREBASE_*`. Firebase Web config và public VAPID key không phải service-account secret.
3. Tải service-account JSON vào `backend/secrets/firebase-service-account.json`. Thư mục này đã nằm trong `.gitignore`; tuyệt đối không commit file đó.
4. Trong `backend/config/local.yml`, đặt `notification.push_enabled: true`, `firebase.project_id` và `firebase.credentials_file: "secrets/firebase-service-account.json"`.
5. Chạy `make migrate`, sau đó restart Notification Service và Gateway.

Storefront/Admin chỉ gọi `Notification.requestPermission()` sau khi người dùng bấm **Bật push**. Khi logout, frontend unregister installation và xóa local FCM token. Web Push production cần HTTPS. Khi chạy Notification Service trong container, mount service-account bằng Docker/Kubernetes secret rồi trỏ `credentials_file` tới đường dẫn read-only trong container; trên Google Cloud có thể để trống `credentials_file` và dùng Application Default Credentials/Workload Identity.

## Redis cache

Book Service cache `GetBook` và từng trang `ListBooks`; Order Service cache `ListCart` theo user. PostgreSQL luôn là source of truth. Mọi thao tác tạo/sửa/xóa sách, reserve/release stock và thay đổi cart đều tăng cache version, nên request tiếp theo dùng key mới thay vì đọc dữ liệu cũ. Key cũ tự hết hạn theo TTL và không cần `SCAN`/xóa hàng loạt trên request path.

Cache dùng TTL jitter, `singleflight` trong process và Redis distributed lock giữa các replica để giảm cache stampede. Redis có timeout ngắn; cache miss hoặc Redis lỗi sẽ fallback PostgreSQL, không làm API ghi hay checkout thất bại. Docker Compose chờ Redis healthy trước khi khởi động Book/Order Service.

Không lưu access token hoặc refresh token thô trong Redis. Access token hiện là JWT stateless; refresh token được hash và rotate/revoke bằng transaction PostgreSQL để chống replay. Nếu sau này cần revoke access token tức thời, nên thêm denylist theo `jti` với TTL bằng thời gian sống còn lại của token, không cache token plaintext.

Kiểm tra cache local:

```bash
docker compose exec redis redis-cli --scan --pattern 'bookstore-local:cache:*'
make e2e-checkout-local
```

## Ghi chú production

- Transactional outbox của Auth và Payment đã dùng PostgreSQL + RabbitMQ publisher confirms. Production nên chạy RabbitMQ cluster ba node, áp policy at-least-once cho DLX, thêm metrics/alert cho pending event và archive bảng outbox.
- Checkout tiền thật đã có VNPAY HMAC webhook, query/refund và settlement reconciliation. Trước khi go-live vẫn phải hoàn thành hợp đồng merchant, dùng production credential/URL, HTTPS public IPN, alert cho mismatch/pending lâu, đối soát báo cáo ngân hàng theo ngày và kiểm thử sandbox/UAT với VNPAY.
- Redis cache là optimization và fail-open; production cần Redis HA/Sentinel hoặc managed Redis, metrics hit/miss/error/latency và giới hạn memory với eviction policy phù hợp. Không dùng Redis cache làm nguồn dữ liệu duy nhất cho cart hoặc payment.
- Access token JWT dùng HMAC và sống `5m`; refresh token opaque sống `168h`, được rotate mỗi lần dùng, revoke theo session family khi phát hiện token cũ bị dùng lại và chỉ lưu hash trong `auth.refresh_sessions`. Production phải bật HTTPS, đặt `refresh_cookie_secure: true`, dùng secret manager cho JWT secret và chỉ whitelist origin storefront thật.
- Khi chuyển Keycloak/Auth0, thay token adapter bằng OIDC/JWKS; application và domain không cần đổi.
- gRPC đang dùng plaintext trong private Docker network. Production nên bật mTLS/service identity.
- `docker-entrypoint-initdb.d` chỉ phù hợp bootstrap local. CI/CD production nên chạy migration job có version riêng.
