# Coworker Production Dockerfile (多阶段构建)
#
# Stage 1: Bun 构建前端
# Stage 2: Go 编译后端（静态链接）
# Stage 3: Debian slim 运行镜像

# ============================================
# Stage 1: 前端构建
# ============================================
FROM oven/bun:latest AS frontend

WORKDIR /build
COPY web/package.json .
COPY web/bun.lock .
RUN bun install
COPY ./web .
RUN DISABLE_ESLINT_PLUGIN='true' bun run build

# ============================================
# Stage 2: 后端编译
# ============================================
FROM golang:1.24-alpine AS backend

ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOPROXY=https://goproxy.cn,https://proxy.golang.org,direct \
    GOTOOLCHAIN=auto \
    GOFLAGS=-mod=mod \
    GONOSUMCHECK=* \
    GONOPROXY= \
    GONOSUMDB=*

ARG TARGETOS
ARG TARGETARCH
ENV GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64}

WORKDIR /build

# 先复制依赖文件，利用 Docker 缓存
COPY go.mod go.sum ./
# 重试3次，解决 goproxy HTTP/2 GOAWAY 问题
RUN for i in 1 2 3; do go mod download && break || echo "Retry $i..." && sleep 5; done

# 复制源码 + 前端构建产物
COPY . .
COPY --from=frontend /build/dist ./web/dist

RUN go build -ldflags "-s -w" -o /tmp/coworker .

# ============================================
# Stage 3: 运行镜像
# ============================================
FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
       ca-certificates tzdata wget curl \
       docker.io \
    && rm -rf /var/lib/apt/lists/* \
    && update-ca-certificates

COPY --from=backend /tmp/coworker /app/coworker

# 创建必要目录
RUN mkdir -p /app/userdata /app/logs /data

EXPOSE 3000
WORKDIR /app
ENTRYPOINT ["/app/coworker"]
