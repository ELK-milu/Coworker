# nsjail 独立容器镜像 (多阶段构建)
# 参考官方 Dockerfile: https://github.com/google/nsjail

# ============================================
# Stage 1: 基础镜像 (运行时依赖)
# ============================================
FROM debian:bookworm-slim AS base

RUN apt-get -y update && apt-get install -y --no-install-recommends \
    libc6 \
    libstdc++6 \
    libprotobuf32 \
    libnl-route-3-200 \
    && rm -rf /var/lib/apt/lists/*

# ============================================
# Stage 2: 编译镜像
# ============================================
FROM base AS build

RUN apt-get -y update && apt-get install -y --no-install-recommends \
    autoconf \
    bison \
    flex \
    gcc \
    g++ \
    git \
    libprotobuf-dev \
    libnl-route-3-dev \
    libtool \
    make \
    pkg-config \
    protobuf-compiler \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# 从 GitHub 克隆并编译 (使用国内镜像)
RUN git clone --depth 1 https://gh-proxy.net/https://github.com/google/nsjail.git /nsjail \
    && cd /nsjail \
    && make

# ============================================
# Stage 3: 运行镜像 (最小化)
# ============================================
FROM base AS run

# 只复制编译好的二进制文件
COPY --from=build /nsjail/nsjail /usr/local/bin/nsjail

# 创建用户数据目录
RUN mkdir -p /userdata

# 保持容器运行 (等待 docker exec 调用)
CMD ["tail", "-f", "/dev/null"]
