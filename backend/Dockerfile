FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG SERVICE
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -trimpath -o /out/service ./cmd/${SERVICE}

FROM alpine:3.22
RUN adduser -D -H -u 10001 appuser \
    && mkdir -p /app/logs \
    && chown -R appuser:appuser /app
WORKDIR /app
USER appuser
COPY --from=builder /out/service /app/service
ENTRYPOINT ["/app/service"]
