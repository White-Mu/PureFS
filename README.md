# PureFS

一个完全开源、可自部署的私有云盘系统。

## 快速开始

### 使用 Docker

```bash
# 构建并启动
docker compose up -d

# 访问 http://localhost:8080
# API: http://localhost:8080/api
# WebDAV: http://localhost:8080/webdav
```

### 手动运行

```bash
# 1. 构建前端
cd PureFS-Web
npm install
npm run build

# 2. 运行后端
cd ../PureFS-App
export GOPROXY=https://goproxy.cn,direct
go run ./cmd/purefsd/

# 首次运行会自动生成 config.yaml，修改后重启
```

### 默认配置

运行后会在当前目录生成 `config.yaml`：

```yaml
server:
  host: 0.0.0.0
  port: 8080
  external_url: http://localhost:8080

database:
  driver: sqlite
  dsn: data/purefs.db?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL

storage:
  driver: local
  local:
    path: data/storage
```

### 注册管理员

启动后通过 API 注册第一个用户：

```bash
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","email":"admin@example.com","password":"your-password"}'
```

然后手动修改数据库将 role 改为 `admin`。

## 项目结构

```
purefs/
├── PureFS-App/          # Go 后端
│   ├── cmd/purefsd/     # 主入口
│   ├── internal/        # 内部包
│   │   ├── auth/        # 认证（密码、TOTP、密码策略）
│   │   ├── config/      # YAML 配置
│   │   ├── crypto/      # AES-256-GCM 加解密 + 密钥轮换
│   │   ├── database/    # SQLite 连接 + goose 迁移
│   │   ├── handler/     # HTTP 处理器（11 个）
│   │   ├── middleware/   # 中间件（认证、CORS、速率限制）
│   │   ├── model/       # 数据模型 + DTO
│   │   ├── monitoring/  # 系统资源监控
│   │   ├── repository/  # 数据访问层（7 个仓库）
│   │   ├── search/      # FTS5 全文搜索 + 文档提取
│   │   ├── service/     # 业务逻辑层（8 个服务）
│   │   ├── storage/     # 存储驱动（本地/S3/加密装饰器）
│   │   └── task/        # 异步任务队列
│   ├── pkg/             # 可复用工具包
│   │   ├── i18n/        # 国际化支持
│   │   ├── jwtutil/     # JWT 工具
│   │   ├── response/    # HTTP 响应工具
│   │   └── sftp/        # SFTP 服务器
│   └── webdav/          # WebDAV 协议支持
├── PureFS-Web/          # React + TypeScript 前端
│   └── src/
│       ├── api/          # API 客户端 + TypeScript 类型
│       ├── components/   # UI 组件（10 个）
│       ├── pages/        # 页面（6 个）
│       ├── store/        # Zustand 状态管理
│       ├── styles/       # CSS 设计令牌 + 组件样式
│       └── i18n/         # 国际化（zh-CN/en-US）
├── docs/                # 设计文档
├── Dockerfile
└── docker-compose.yml
```

## 核心特性

- **原生文件存储**：文件直接以原生目录结构存储，无私有封装格式。程序损坏也能直接拷硬盘带走
- **标准协议**：原生支持 WebDAV 和 SFTP，电脑手机 NAS 可直接挂载
- **多用户权限**：基于文件夹的读写权限分配
- **分享外链**：支持有效期、提取码、访问次数限制、下载权限
- **双因素认证**：TOTP 2FA 支持
- **存储后端**：本地磁盘 / S3 兼容对象存储
- **多种浏览**：列表 / 大图标网格 / 时间线
- **文件管理**：上传、下载、重命名、移动、批量删除、置顶、收藏、行内重命名、右键菜单
- **回收站**：文件删除先进回收站，30 天自动清理
- **文件版本**：上传覆盖时自动保存旧版本，支持恢复
- **全文搜索**：SQLite FTS5 搜索引擎，支持文档内容检索
- **透明加密**：可选的 AES-256-GCM 存储加密，对上层完全透明，支持密钥轮换
- **操作日志**：完整审计日志
- **文件完整性**：定期 SHA256 校验，防止数据静默损坏
- **深色模式**：低饱和度浅色 + 暗色模式，侧边栏一键切换
- **管理后台**：系统资源监控、用户管理、权限管理、一键备份
- **速率限制 + 密码策略**：可配置的速率限制和密码强度要求
- **零广告**：无任何广告、弹窗、冗余推荐
- **国际化**：中/英双语支持（基础设施已就绪）

## 插件系统（开发中）

插件通过 gRPC sidecar 进程与核心通信，计划提供：
- 相册智能整理
- 在线文档编辑
- 离线下载
- OCR 图片文字识别

## 桌面客户端（开发中）

基于 Wails 框架，支持 Win/Mac/Linux，计划提供：
- 增量同步
- 选择性同步
- 虚拟盘模式

## 移动客户端（开发中）

基于 React Native，支持 iOS/Android，计划提供：
- 相册自动备份
- 文件离线缓存

## 许可证

MIT
