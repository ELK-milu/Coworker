# nsjail 独立容器镜像 (多阶段构建)
# 参考官方 Dockerfile: https://github.com/google/nsjail

# ============================================
# Stage 1: 编译镜像 (基于 Debian)
# ============================================
FROM debian:bookworm-slim AS build

RUN apt-get -y update && apt-get install -y --no-install-recommends \
    autoconf \
    bison \
    flex \
    gcc \
    g++ \
    git \
    libc6 \
    libstdc++6 \
    libprotobuf32 \
    libprotobuf-dev \
    libnl-route-3-200 \
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
# Stage 2: 运行镜像 (基于 Python 官方镜像)
# ============================================
FROM python:3.11-slim-bookworm AS run

# 安装 nsjail 运行时依赖
# 注意: opencv-python-headless 不需要 libgl1 等 GUI 库
RUN apt-get -y update && apt-get install -y --no-install-recommends \
    libc6 \
    libstdc++6 \
    libprotobuf32 \
    libnl-route-3-200 \
    bash \
    coreutils \
    libglib2.0-0 \
    && rm -rf /var/lib/apt/lists/*

# 配置 pip 使用国内镜像
RUN pip config set global.index-url https://pypi.tuna.tsinghua.edu.cn/simple

# 安装常用 Python 包
# 图片处理
RUN pip install --no-cache-dir \
    Pillow \
    opencv-python-headless \
    imageio

# 办公文件处理
RUN pip install --no-cache-dir \
    openpyxl \
    python-docx \
    PyPDF2 \
    xlrd \
    xlwt \
    python-pptx \
    chardet

# 数据计算
RUN pip install --no-cache-dir \
    numpy \
    pandas \
    scipy \
    sympy \
    matplotlib

# 其他常用工具
RUN pip install --no-cache-dir \
    requests \
    beautifulsoup4 \
    lxml \
    pyyaml \
    python-dateutil

# 只复制编译好的二进制文件
COPY --from=build /nsjail/nsjail /usr/local/bin/nsjail

# 创建用户数据目录
RUN mkdir -p /userdata

# 保持容器运行 (等待 docker exec 调用)
CMD ["tail", "-f", "/dev/null"]
