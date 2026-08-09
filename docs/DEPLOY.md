# PureFS 部署文档

## 1. 部署方式

PureFS 支持两种部署方式：

- **Docker / Docker Compose**（推荐）
- **裸机二进制**（直接安装在系统上）

## 2. Docker 部署

### 2.1 快速启动

```yaml
# docker-compose.yml
services:
  purefs:
    image: purefs/purefs:latest
    ports:
      - "8080:8080"    # Web 管理界面
      - "8081:8081"    # WebDAV
      - "8022:22"      # SFTP
    volumes:
      - ./data:/data/purefs     # 数据目录（文件 + 数据库 + 配置）
      - ./storage:/storage      # 文件存储目录
    environment:
      - PUREFS_DATA_DIR=/data/purefs
      - PUREFS_STORAGE_DIR=/storage
      - PUREFS_JWT_SECRET=change-me
    restart: always
```

### 2.2 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `PUREFS_DATA_DIR` | 数据目录（包含数据库、配置、索引） | `/data/purefs` |
| `PUREFS_STORAGE_DIR` | 文件存储目录 | `/data/purefs/storage` |
| `PUREFS_JWT_SECRET` | JWT 签名密钥 | 必填 |
| `PUREFS_ENCRYPTION_KEY` | 加密主密钥 | 留空则不启用加密 |
| `PUREFS_PORT` | HTTP 端口 | `8080` |
| `PUREFS_LOG_LEVEL` | 日志级别 | `info` |

## 3. 裸机部署

### 3.1 下载与安装

```bash
# 下载对应平台二进制
wget https://github.com/purefs/purefs/releases/latest/download/purefs-linux-amd64.tar.gz
tar xzf purefs-linux-amd64.tar.gz
sudo install purefsd /usr/local/bin/

# 创建配置目录
sudo mkdir -p /etc/purefs /var/lib/purefs

# 编辑配置
sudo nano /etc/purefs/config.yaml
```

### 3.2 systemd 服务

```ini
[Unit]
Description=PureFS Private Cloud
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/purefsd --config /etc/purefs/config.yaml
Restart=always
User=purefs
Group=purefs

[Install]
WantedBy=multi-user.target
```

## 4. 配置文件

```yaml
# /etc/purefs/config.yaml
server:
  port: 8080
  webdav_port: 8081
  sftp_port: 8022
  data_dir: /var/lib/purefs

storage:
  driver: local
  local:
    path: /var/lib/purefs/storage
  # s3:
  #   endpoint: https://s3.amazonaws.com
  #   region: us-east-1
  #   bucket: purefs
  #   access_key: ""
  #   secret_key: ""

database:
  driver: sqlite
  sqlite:
    path: /var/lib/purefs/purefs.db
    wal_mode: true
  # postgres:
  #   dsn: postgres://user:pass@localhost/purefs?sslmode=disable

auth:
  jwt_secret: change-me-please
  jwt_expiry: 24h
  enable_2fa: true

encryption:
  enabled: false
  key: ""

search:
  engine: bleve
  bleve:
    index_path: /var/lib/purefs/search.bleve
    memory_limit_mb: 128

integrity:
  enabled: true
  fast_scan_interval: 24h    # xxHash 快速扫描间隔
  deep_scan_interval: 168h   # SHA256 深度扫描间隔 (7天)
  algorithm: xxhash

trash:
  retention_days: 30

log:
  level: info
  audit_enabled: true
```

## 5. 最低配置要求

| 规格 | 配置 |
|------|------|
| CPU | 1 核 |
| 内存 | 256 MB（最小），512 MB（推荐） |
| 磁盘 | 取决于存储文件体积 |
| 操作系统 | Linux (amd64/arm64/armv7), macOS, Windows |
