# syntax=docker/dockerfile:1
FROM golang:1.24-alpine AS builder

WORKDIR /app

# 使用阿里云镜像源
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    apk add --no-cache gcc musl-dev

COPY go.mod go.sum ./
# 使用 BuildKit 缓存模块与编译产物，依赖未变化时不重复下载和全量编译。
RUN --mount=type=cache,target=/go/pkg/mod \
    GOPROXY=https://goproxy.cn,direct go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -o main .

FROM alpine:latest

# 使用阿里云镜像源
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/main .
COPY --from=builder /app/etc ./etc
COPY --from=builder /app/ssl ./ssl

RUN mkdir -p /app/storage/logs /app/storage/cache

EXPOSE 8501

ENV TZ=Asia/Shanghai

CMD ["./main", "server"]
