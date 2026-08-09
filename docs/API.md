# PureFS RESTful API 定义

## 1. 通用约定

- 基础路径：`/api/v1`
- 请求/响应格式：`application/json`
- 认证方式：`Authorization: Bearer <JWT>`
- 分页：`?offset=0&limit=50`（默认 limit=50，最大 500）
- 时间格式：ISO 8601
- 错误响应：`{ "error": { "code": "...", "message": "..." } }`

## 2. 认证 API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/auth/register` | 用户注册 |
| POST | `/auth/login` | 用户登录，返回 JWT |
| POST | `/auth/refresh` | 刷新 Token |
| POST | `/auth/logout` | 登出 |
| POST | `/auth/2fa/enable` | 启用 2FA（返回 TOTP 密钥） |
| POST | `/auth/2fa/verify` | 验证 2FA 并完成绑定 |
| POST | `/auth/2fa/disable` | 禁用 2FA |

### POST /auth/register

```json
{ "username": "string", "password": "string", "display_name": "string (optional)" }
→ { "user": { "id": "...", "username": "...", "display_name": "..." } }
```

### POST /auth/login

```json
{ "username": "string", "password": "string", "totp_code": "string (optional)" }
→ { "token": "jwt...", "user": { ... }, "need_2fa": false }
```

## 3. 文件 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/files` | 列出文件/目录（默认按时间降序） |
| GET | `/files/:path` | 获取文件/目录详情 |
| POST | `/files` | 上传文件 |
| POST | `/files/mkdir` | 创建目录 |
| DELETE | `/files/:path` | 删除文件/目录（移入回收站） |
| PATCH | `/files/:path/rename` | 重命名 |
| PATCH | `/files/:path/move` | 移动 |
| POST | `/files/batch/delete` | 批量删除 |
| POST | `/files/batch/move` | 批量移动 |
| POST | `/files/batch/rename` | 批量重命名 |
| POST | `/files/copy` | 复制 |

### GET /files

```
Query: parent=xxx&sort=created_at&order=desc&view=list|grid|timeline&offset=0&limit=50
→ {
    "items": [
      {
        "name": "string",
        "path": "string",
        "is_dir": false,
        "size": 1234,
        "mime_type": "image/png",
        "created_at": "2026-07-29T10:00:00Z",
        "modified_at": "2026-07-29T10:00:00Z",
        "pinned": false,
        "favorited": false,
        "thumbnail_url": "string (optional)"
      }
    ],
    "total": 100,
    "offset": 0,
    "limit": 50
  }
```

### POST /files (upload)

```
Content-Type: multipart/form-data
Fields: file, path (目标目录), auto_rename (可选)
→ { "file": { ... } }
```

## 4. 收藏/置顶 API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/favorites` | 添加收藏 |
| DELETE | `/favorites/:path` | 取消收藏 |
| GET | `/favorites` | 获取收藏列表 |
| POST | `/pins` | 置顶文件 |
| DELETE | `/pins/:path` | 取消置顶 |
| PATCH | `/pins/reorder` | 调整置顶顺序 |

## 5. 搜索 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/search` | 全局搜索 |

### GET /search

```
Query: q=xxx&type=all|file|document|image&offset=0&limit=50
→ { "items": [...], "total": ... }
```

## 6. 分享 API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/shares` | 创建分享 |
| GET | `/shares` | 获取我的分享列表 |
| DELETE | `/shares/:id` | 取消分享 |
| PATCH | `/shares/:id` | 更新分享配置 |
| GET | `/s/:share_id` | 访问分享（公开） |
| POST | `/s/:share_id/verify` | 验证提取码 |

### POST /shares

```json
{
  "file_path": "/photos/vacation",
  "password": "optional pin",
  "expire_days": 7,
  "max_access_count": 100,
  "allow_download": true
}
→ { "share": { "id": "...", "url": "https://...", ... } }
```

## 7. 用户管理 API（管理员）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/users` | 用户列表 |
| POST | `/admin/users` | 创建用户 |
| PATCH | `/admin/users/:id` | 修改用户 |
| DELETE | `/admin/users/:id` | 删除用户 |
| GET | `/admin/stats` | 系统资源统计 |

## 8. 权限 API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/permissions` | 设置文件夹权限 |
| GET | `/permissions` | 获取文件夹权限 |
| DELETE | `/permissions/:id` | 移除权限 |

## 9. 回收站 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/recycle` | 回收站列表 |
| POST | `/recycle/:id/restore` | 恢复文件 |
| DELETE | `/recycle/:id` | 永久删除 |
| DELETE | `/recycle` | 清空回收站 |

## 10. 管理后台 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/system/status` | CPU/内存/磁盘状态 |
| GET | `/admin/system/logs` | 系统日志 |
| POST | `/admin/system/backup` | 触发备份 |
| POST | `/admin/system/update` | 一键更新 |
| GET | `/admin/storage` | 存储后端配置 |
| PUT | `/admin/storage` | 修改存储后端配置 |
| GET | `/admin/plugins` | 插件列表 |
| PATCH | `/admin/plugins/:id` | 启用/禁用插件 |

## 11. 审计日志 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/logs` | 审计日志列表 |

```
Query: user_id=xxx&action=xxx&from=xxx&to=xxx&offset=0&limit=50
→ { "items": [{ ... }], "total": ... }
```
