# ---- 构建阶段 ----
FROM golang:1.24 as builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# 动态编译（允许 CGO）
RUN GOOS=linux go build -ldflags '-s -w' -o go-proxy ./main.go

FROM debian:stable-slim
WORKDIR /app

# 安装 SQLite3 依赖（Debian 自带 glibc）
RUN apt-get update && apt-get install -y --no-install-recommends \
    sqlite3 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/go-proxy .
# COPY data/config.yaml ./data/config.yaml
COPY public ./public

EXPOSE 8080
CMD ["/app/go-proxy"]