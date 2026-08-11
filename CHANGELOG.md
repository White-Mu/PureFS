# Changelog

本项目的所有重要变更都会记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### 计划中
- 插件系统（gRPC sidecar）
- 桌面客户端（Wails）
- 移动客户端（React Native）
- 测试覆盖（目标 80%）
- 性能优化（虚拟滚动、缩略图、懒加载）

## [v0.2.0] - 2026-08-11

### 新增
- **端到端加密（E2EE）**：客户端 WebCrypto 加解密，PBKDF2-SHA256（15 万次迭代）派生口令密钥，AES-256-GCM 信封加密。服务器只存储密文，忘记口令数据不可恢复。
  - 迁移 `008_e2ee.sql`：`users.e2ee_salt`、`users.e2ee_wrapped_key`、`files.is_e2ee`
  - E2EE 设置/状态/禁用 API：`GET/POST/DELETE /api/users/e2ee`
  - 上传支持 `is_e2ee` + 包装 DEK，下载/预览自动解密
  - E2EE 文件自动跳过全文索引
  - 文件复制保留包装 DEK，副本可解密
- **HTTPS/TLS 支持**：配置 `tls` 段（`enabled`/`cert`/`key`/`auto`），`auto` 模式首次启动自动生成自签证书（ECDSA P-256）
- **E2EE 设置卡片中英双语**：设置页新增端到端加密管理卡片（开启/关闭/解锁主密钥/备份密钥）

### 修复
- 审计日志从未真正接线：`auditSvc` 创建后无人调用 `Log()`，现在已注入 UserService / FileService，登录、注册、上传、下载、删除、重命名、复制、建目录均有记录
- `userRepo.Update()` SQL 缺少 `password_hash` 列，导致密码重置后无法登录
- Admin 页面空白：空表返回 nil Go slice → JSON `null` → 前端 `.map()` 崩溃，改为返回 `[]`

### 变更
- P2 全部完成（100%）：i18n、响应式 CSS、密码重置、图片缩略图、文件复制、批量删除、Refresh Token、管理员用户管理、本地 QR 码生成

## [v0.1.0] - 2026-08-09

PureFS 首个可用版本。

### 新增
- **核心文件管理**：上传、下载、重命名、移动、删除、批量操作、行内重命名、右键菜单
- **三种浏览视图**：列表 / 大图标网格 / 时间线
- **认证**：JWT 登录 + 注册，TOTP 双因素认证（兼容 Google Authenticator / Authy）
- **分享外链**：密码、有效期、访问次数限制、下载权限，公开落地页内联预览
- **存储后端**：本地磁盘（LocalFS）/ S3 兼容对象存储
- **WebDAV + SFTP**：原生协议支持，用户空间隔离 + 数据库同步
- **多用户权限**：基于文件夹的 read/write/admin 权限，最长前缀匹配
- **收藏 / 置顶 / 最近**：侧边栏导航，SQL 层过滤
- **回收站**：软删除 / 恢复 / 清空，30 天自动清理
- **文件版本**：上传覆盖自动保存旧版本，可恢复（默认保留 10 份）
- **全文搜索**：SQLite FTS5 + 文档内容提取 + 异步索引任务队列
- **服务端透明加密**：AES-256-GCM + HKDF 密钥轮换（可选开启）
- **文件完整性校验**：定期 SHA256 校验，检测静默损坏
- **管理后台**：系统资源监控（CPU/内存/磁盘 + SSE）、用户管理、权限管理、一键备份、审计日志
- **速率限制 + 密码策略**：可配置登录/注册限流与密码强度要求
- **暗色模式**：低饱和度浅色 + 暗色主题，侧边栏一键切换
- **Docker 部署**：多阶段构建（x86_64 + ARM64）+ Compose 一键启动

### 技术栈
- 后端：Go + chi 路由器 + SQLite（modernc.org，无 CGO）
- 前端：React 19 + TypeScript + Vite + Zustand
