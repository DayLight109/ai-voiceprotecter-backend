# ───────── build ─────────
FROM golang:1.22-alpine AS builder

WORKDIR /src

# 缓存依赖
COPY go.mod go.sum* ./
RUN go mod download || true

# 复制源码
COPY . .

# 编译
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/server ./cmd/server

# ───────── runtime ───────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S app && adduser -S app -G app

WORKDIR /app
COPY --from=builder /out/server /app/server

USER app
EXPOSE 8080

ENTRYPOINT ["/app/server"]
CMD ["-addr=:8080"]
