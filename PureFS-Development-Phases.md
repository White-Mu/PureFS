# PureFS 当前状态总览

> 更新时间: 2026-07-30

## 代码规模

| 层级 | 文件数 | 代码行数 |
|------|--------|----------|
| Go 后端 | 55 个 .go 文件 | ~7,900 行 |
| React 前端 | 14 个 .tsx/.ts 文件 | ~2,000 行 |
| CSS 样式 | 2 个 .css 文件 | ~790 行 |
| **合计** | **71 个源文件** | **~10,700 行** |

## 后端架构

```
PureFS-App/
├── cmd/purefsd/main.go            # 主入口，依赖注入，启动流程
├── internal/
│   ├── config/config.go           # YAML 配置，首次运行自动生成
│   ├── database/                  # SQLite (modernc) + goose 迁移
│   │   └── migrations/            # 6 个迁移文件
│   ├── model/                     # 数据模型 + DTO
│   ├── repository/                # 7 个仓库 (user, file, share, permission, audit, recycle, version)
│   ├── service/                   # 7 个服务 (file, user, share, audit, recycle, integrity, version, search)
│   ├── handler/                   # 10 个处理器 (file, user, share, public_share, audit, permission, recycle, search, version, admin)
│   ├── middleware/                 # 认证、CORS、速率限制、日志
│   ├── storage/                   # 存储抽象 (local, s3, encrypted 装饰器)
│   ├── crypto/                    # AES-256-GCM 加解密 + 密钥轮换
│   ├── search/                    # SQLite FTS5 全文搜索 + 文档提取
│   ├── task/queue.go              # 异步任务队列 (goroutine + channel)
│   ├── monitoring/stats.go        # 系统资源监控
│   └── plugin/                    # (待实现)
├── pkg/
│   ├── jwtutil/                   # JWT 签发/解析
│   ├── response/                  # HTTP 响应工具
│   └── sftp/                      # SFTP 服务器
├── webdav/webdav.go              # WebDAV 协议支持
└── plugins/sdk/                   # 插件 SDK (待实现)
```

## 前端架构

```
PureFS-Web/
└── src/
    ├── api/
    │   ├── client.ts              # Axios 客户端 + JWT 拦截器
    │   └── index.ts               # API 函数 + TypeScript 类型
    ├── components/
    │   ├── AppLayout.tsx           # (未使用)
    │   ├── Breadcrumb.tsx          # 面包屑导航
    │   ├── ContextMenu.tsx         # 右键菜单 (Portal 实现)
    │   ├── FileGrid.tsx            # 网格视图
    │   ├── FilePreview.tsx         # 文件预览 (图片/视频/文本)
    │   ├── FileRow.tsx             # 列表行 + 行内重命名
    │   ├── SelectionToolbar.tsx    # 批量操作栏
    │   ├── ShareDialog.tsx         # 分享链接创建弹窗
    │   ├── Sidebar.tsx             # 左侧导航
    │   └── UploadOverlay.tsx       # 上传进度浮层
    ├── pages/
    │   ├── LoginPage.tsx           # 登录/注册/2FA
    │   ├── FilesPage.tsx           # 核心文件浏览器 (含 TimelineView)
    │   ├── SharePage.tsx           # 公开分享页面
    │   ├── SharesPage.tsx          # 我的分享列表
    │   ├── SettingsPage.tsx        # 用户设置/2FA 配置
    │   └── AdminPage.tsx           # 管理后台 (审计/用户/权限)
    ├── store/index.ts             # Zustand (auth + UI 状态)
    ├── styles/
    │   ├── globals.css             # 设计令牌 (浅色/暗色主题)
    │   └── components.css          # 组件样式
    └── i18n/                      # 国际化 (zh-CN / en-US)
```

## 已实现功能清单

### 文件管理
- [x] 文件 CRUD (上传/下载/预览/重命名/移动/删除)
- [x] 三种浏览视图 (列表 / 大图标网格 / 时间线)
- [x] 置顶 & 收藏标记 + 专用页面
- [x] 批量选择 & 批量删除
- [x] 右键上下文菜单 (刷新/收藏/置顶/重命名/下载/分享/删除)
- [x] 行内重命名 (Enter/✓ 确认, Escape/✕ 取消)
- [x] 拖拽上传
- [x] 文件夹导航 (面包屑)
- [x] 默认按时间降序排列

### 用户与安全
- [x] 用户注册 & 登录 (JWT + bcrypt)
- [x] TOTP 双因素认证
- [x] 基于文件夹路径的权限 (read/write/admin)
- [x] 密码强度校验
- [x] 速率限制 (登录/注册/上传)
- [x] AES-256-GCM 服务端透明加密 + 密钥轮换
- [x] 完整审计日志
- [x] 零数据扫描、零数据分析

### 分享
- [x] 生成分享外链
- [x] 提取码 (密码)
- [x] 有效期设置
- [x] 访问次数限制
- [x] 下载权限控制

### 搜索
- [x] 文件名搜索
- [x] SQLite FTS5 全文搜索 (文档内容)
- [x] 异步索引任务队列
- [x] 文档文本提取 (txt/md/pdf/docx/xlsx/pptx)

### 回收站
- [x] 删除先进回收站
- [x] 30 天自动清理
- [x] 恢复 & 永久删除

### 文件版本
- [x] 上传覆盖自动保存旧版本
- [x] 版本列表/恢复/删除 API
- [x] 可配置保留版本数 (默认 10)

### 协议支持
- [x] WebDAV (用户空间隔离 + DB 同步)
- [x] SFTP (嵌入式 SSH 服务器)

### 存储
- [x] 本地磁盘 (原生文件夹结构)
- [x] S3 兼容对象存储
- [x] 完整性校验 (SHA256, 定期自动)
- [x] 存储配额强制

### 管理后台
- [x] 审计日志查看
- [x] 用户管理
- [x] 权限管理
- [x] 系统资源监控 (CPU/内存/磁盘)
- [x] 一键备份 (SQLite VACUUM INTO)
- [x] SSE 实时统计推送

### UI
- [x] 低饱和度浅色风格 (默认)
- [x] 暗色模式切换 (CSS 自定义属性)
- [x] 侧边栏导航
- [x] 响应式空状态提示
- [x] 零广告、零弹窗、零推荐
- [x] 国际化基础设施 (中/英文地区文件)

### 部署
- [x] Docker 多阶段构建 (x86_64 + ARM64)
- [x] Docker Compose 一键部署
- [x] 裸机二进制支持

## 待实现功能

- [ ] 插件系统 (gRPC sidecar)
- [ ] 桌面客户端 (Wails: 增量同步 + 虚拟盘)
- [ ] 移动客户端 (React Native: 相册备份 + 离线缓存)
- [ ] 测试覆盖 (目标 80%)
- [ ] 性能优化 (虚拟滚动、缩略图、懒加载)
