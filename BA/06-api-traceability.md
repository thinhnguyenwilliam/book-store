# Truy vết yêu cầu — API

Base REST path là `/api/v1`. `Public` không cần access token; `Customer` cần xác thực và ownership; `Admin` cần role admin.

## Authentication

- `POST /auth/register` — Public — `FR-AUTH-001`.
- `POST /auth/login` — Public — `FR-AUTH-002`.
- `POST /auth/provider-state` — Public — `FR-AUTH-003`, `FR-AUTH-004`.
- `POST /auth/google` — Public — `FR-AUTH-003`, `FR-AUTH-005`.
- `POST /auth/facebook` — Public — `FR-AUTH-004`, `FR-AUTH-005`.
- `POST /auth/refresh` — Refresh cookie — `FR-AUTH-006`.
- `POST /auth/logout` — Refresh cookie — `FR-AUTH-007`.

## User và admin customer

- `GET /users/me` — Customer/Admin — `FR-USER-001`.
- `PUT /users/me` — Customer/Admin — `FR-USER-002`.
- `GET /admin/customers` — Admin — `FR-USER-003`.
- `GET /admin/customers/:id` — Admin — `FR-USER-004`.
- `PUT /admin/customers/:id` — Admin — `FR-USER-004`.
- `DELETE /admin/customers/:id` — Admin, trả `202` — `FR-USER-005`.

## Books

- `GET /books?limit=&cursor=` — Public — `FR-BOOK-001`.
- `GET /books/:id` — Public — `FR-BOOK-002`.
- `POST /admin/books` — Admin — `FR-BOOK-003`.
- `PUT /admin/books/:id` — Admin — `FR-BOOK-004`.
- `DELETE /admin/books/:id` — Admin — `FR-BOOK-005`.

## Cart và order

- `GET /cart/items` — Customer — `FR-CART-001`.
- `POST /cart/items` — Customer — `FR-CART-002`.
- `PUT /cart/items/:id` — Customer — `FR-CART-003`.
- `DELETE /cart/items/:id` — Customer — `FR-CART-004`.
- `DELETE /cart/items` hoặc `POST /cart/items/batch-delete` — Customer — `FR-CART-004`.
- `POST /orders` — Customer, cần `Idempotency-Key` — `FR-ORDER-001`, `FR-ORDER-002`.
- `GET /orders?limit=&cursor=` — Customer — `FR-ORDER-003`.
- `GET /orders/:id` — Customer/owner — `FR-ORDER-003`.
- `PUT /orders/:id/cancel` — Customer/owner — `FR-ORDER-004`.

## Payment và wallet

- `POST /wallets/me` — Customer — `FR-PAY-001`.
- `GET /wallets/me` — Customer — `FR-PAY-001`.
- `PUT /admin/wallets/:owner_id/balance` — Admin, cần `Idempotency-Key` — `FR-PAY-002`.
- `POST /payments` — Customer/owner, cần `Idempotency-Key` — `FR-PAY-003`.
- `GET /payments/:id` — Customer/owner — `FR-PAY-004`.
- `GET /payments/order/:order_id` — Customer/owner — `FR-PAY-004`.
- `GET /payments/webhooks/vnpay` — VNPAY IPN — `FR-PAY-006`.

## Comment

- `GET /books/:id/comments?limit=&cursor=` — Public — `FR-CMT-001`.
- `GET /comments/:id/replies?limit=&cursor=` — Public — `FR-CMT-001`.
- `POST /books/:id/comments` — Customer — `FR-CMT-002`.
- `PUT /comments/:id` — Customer/author — `FR-CMT-003`.
- `DELETE /comments/:id` — Customer/author hoặc Admin — `FR-CMT-004`.
- `PUT /admin/comments/:id/status` — Admin — `FR-CMT-005`.

## Chat

- `POST /chat/conversations/support` — Customer — `FR-CHAT-001`.
- `GET /chat/conversations?limit=&cursor=` — Customer/Admin — `FR-CHAT-002`.
- `GET /chat/conversations/:id/messages?limit=&cursor=` — Member/Admin — `FR-CHAT-005`.
- `POST /chat/conversations/:id/messages` — Member/Admin — `FR-CHAT-003`.
- `PUT /chat/messages/:id` — Sender — `FR-CHAT-004`.
- `DELETE /chat/messages/:id` — Sender/Admin — `FR-CHAT-004`.
- `PUT /chat/conversations/:id/read` — Member/Admin — `FR-CHAT-005`.
- `GET /chat/unread-count` — Customer/Admin — `FR-CHAT-006`.
- `POST /chat/ws-ticket` — Customer/Admin — `FR-CHAT-006`.
- `GET /chat/ws?ticket=` — Ticket ngắn hạn — `FR-CHAT-006`.

## Notification

- `GET /notifications?limit=&cursor=` — Customer/Admin — `FR-NOTI-001`.
- `GET /notifications/unread-count` — Customer/Admin — `FR-NOTI-001`.
- `PUT /notifications/:id/read` — Owner — `FR-NOTI-002`.
- `PUT /notifications/read-all` — Owner — `FR-NOTI-002`.
- `POST /notifications/devices` — Customer/Admin — `FR-NOTI-003`.
- `DELETE /notifications/devices/:device_id` — Device owner — `FR-NOTI-003`.

## GraphQL

Endpoint duy nhất là `POST /graphql`:

- `bookDetail` — Public — `FR-GQL-001`.
- `adminDashboard` — Admin — `FR-GQL-002`.
- `orderDetail` — Customer/owner — `FR-GQL-003`.

## Ghi chú lỗi transport

Gateway map lỗi gRPC sang HTTP thống nhất: invalid input `400`, unauthenticated `401`, forbidden `403`, not found `404`, conflict `409`, rate limit `429`, deadline `504`, unavailable `503`. Internal error không được lộ nguyên văn.
