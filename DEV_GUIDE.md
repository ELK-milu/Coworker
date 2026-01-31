# 开发环境使用指南

## 快速开始

### 1. 配置环境变量

创建 `.env` 文件（可以从 `.env.example` 复制）：

```bash
cp .env.example .env
```

编辑 `.env` 文件，设置必要的环境变量：

```env
ANTHROPIC_API_KEY=your-api-key-here
```

### 2. 启动开发环境

使用开发版 docker-compose 启动：

```bash
docker-compose -f docker-compose-dev.yml up
```

或者后台运行：

```bash
docker-compose -f docker-compose-dev.yml up -d
```

### 3. 查看日志

```bash
docker-compose -f docker-compose-dev.yml logs -f new-api-dev
```

## 开发特性

### 热重载

开发环境使用 [air](https://github.com/cosmtrek/air) 实现热重载：

- 修改 `.go` 文件后自动重新编译
- 修改模板文件（`.tpl`, `.tmpl`, `.html`）后自动重启
- 编译错误会显示在日志中

### 代码挂载

源代码目录实时挂载到容器：

- 本地修改立即生效
- 无需重新构建镜像
- 支持 IDE 调试

## 常用命令

### 停止服务

```bash
docker-compose -f docker-compose-dev.yml down
```

### 重启服务

```bash
docker-compose -f docker-compose-dev.yml restart new-api-dev
```

### 进入容器

```bash
docker exec -it new-api-dev sh
```

## 数据库访问

### PostgreSQL

开发环境的 PostgreSQL 端口已映射到本地：

```bash
# 连接信息
Host: localhost
Port: 5432
Database: new-api
Username: root
Password: 123456
```

使用 psql 连接：

```bash
docker exec -it postgres-dev psql -U root -d new-api
```

### Redis

Redis 端口已映射到本地：

```bash
# 连接信息
Host: localhost
Port: 6379
```

使用 redis-cli 连接：

```bash
docker exec -it redis-dev redis-cli
```

## 故障排查

### 端口冲突

如果端口被占用，修改 `docker-compose-dev.yml` 中的端口映射。

### 热重载不工作

检查 `.air.toml` 配置，确保文件扩展名在 `include_ext` 中。
