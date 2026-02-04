# Coworker Sandbox Image
# 用于 AI 编程助手的安全沙箱环境
# 支持 Python 前后端开发

FROM python:3.12-slim

# 使用国内镜像源
RUN sed -i 's/deb.debian.org/mirrors.aliyun.com/g' /etc/apt/sources.list.d/debian.sources && \
    sed -i 's/security.debian.org/mirrors.aliyun.com/g' /etc/apt/sources.list.d/debian.sources

# 安装系统依赖
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    curl \
    bash \
    && rm -rf /var/lib/apt/lists/*

# 配置 pip 国内镜像源
RUN pip config set global.index-url https://mirrors.aliyun.com/pypi/simple/ && \
    pip config set global.trusted-host mirrors.aliyun.com

# 复制 requirements 文件
COPY requirements.txt /tmp/requirements.txt

# 安装 Python 包
RUN pip install --no-cache-dir -r /tmp/requirements.txt && \
    rm /tmp/requirements.txt

# 创建非 root 用户
RUN useradd -m -s /bin/bash -u 1000 sandbox

# 设置工作目录
WORKDIR /workspace

# 切换到非 root 用户
USER sandbox

# 默认命令
CMD ["sleep", "infinity"]
