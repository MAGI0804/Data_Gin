FROM golang:1.24-alpine AS builder

WORKDIR /app

# 使用阿里云镜像源
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    apk add --no-cache gcc musl-dev

COPY go.mod go.sum ./
# 使用 goproxy.cn 加速 Go 模块下载
RUN GOPROXY=https://goproxy.cn,direct go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

FROM alpine:latest

# 使用阿里云镜像源
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/main .
COPY --from=builder /app/etc ./etc
COPY --from=builder /app/ssl ./ssl

RUN mkdir -p /app/storage/logs /app/storage/cache

EXPOSE 443 8501

ENV TZ=Asia/Shanghai

CMD ["./main", "server"]
