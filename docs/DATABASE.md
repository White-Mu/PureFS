# PureFS 数据库设计

## 1. 总体约束

- 默认使用 **SQLite（WAL 模式）**，可选 **PostgreSQL**
- 全部表使用 `INTEGER PRIMARY KEY AUTOINCREMENT` 作为主键
- 文件元数据以文件系统为准，数据库只存索引和业务关联

## 2. 表结构

### 2.1 users — 用户表

创建于迁移 `001_init`。

```sql
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'user',
    totp_secret TEXT NOT NULL DEFAULT '',
    totp_enabled INTEGER NOT NULL DEFAULT 0,
    storage_quota INTEGER NOT NULL DEFAULT 10737418240,
    storage_used INTEGER NOT NULL DEFAULT 0,
    root_dir TEXT NOT NULL DEFAULT '/',
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### 2.2 files — 文件/目录索引

创建于迁移 `001_init`。

`is_pinned` 和 `is_favorite` 作为列直接存放在 files 表中，不再使用独立的 favorites / pinned_items 表。

```sql
CREATE TABLE IF NOT EXISTS files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_id INTEGER REFERENCES files(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    path TEXT NOT NULL,
    real_path TEXT NOT NULL DEFAULT '',
    file_type TEXT NOT NULL DEFAULT 'file',
    mime_type TEXT NOT NULL DEFAULT '',
    size INTEGER NOT NULL DEFAULT 0,
    sha256 TEXT NOT NULL DEFAULT '',
    is_pinned INTEGER NOT NULL DEFAULT 0,
    is_favorite INTEGER NOT NULL DEFAULT 0,
    is_encrypted INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, parent_id, name)
);
CREATE INDEX IF NOT EXISTS idx_files_user_id ON files(user_id);
CREATE INDEX IF NOT EXISTS idx_files_parent_id ON files(parent_id);
CREATE INDEX IF NOT EXISTS idx_files_path ON files(path);
CREATE INDEX IF NOT EXISTS idx_files_name ON files(name);
CREATE INDEX IF NOT EXISTS idx_files_pinned ON files(is_pinned);
CREATE INDEX IF NOT EXISTS idx_files_favorite ON files(is_favorite);
```

### 2.3 shares — 分享外链

创建于迁移 `001_init`。

```sql
CREATE TABLE IF NOT EXISTS shares (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    file_id INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    token TEXT NOT NULL UNIQUE,
    password TEXT NOT NULL DEFAULT '',
    expires_at DATETIME,
    max_accesses INTEGER,
    access_count INTEGER NOT NULL DEFAULT 0,
    can_download INTEGER NOT NULL DEFAULT 1,
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_shares_token ON shares(token);
```

### 2.4 permissions — 文件夹权限

创建于迁移 `001_init`。

```sql
CREATE TABLE IF NOT EXISTS permissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    file_path TEXT NOT NULL,
    perm TEXT NOT NULL DEFAULT 'read',
    UNIQUE(user_id, file_path)
);
```

### 2.5 audit_logs — 操作审计日志

创建于迁移 `001_init`。

```sql
CREATE TABLE IF NOT EXISTS audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    action TEXT NOT NULL,
    detail TEXT NOT NULL DEFAULT '',
    ip TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);
```

### 2.6 integrity_records — 文件完整性记录

创建于迁移 `001_init`。

```sql
CREATE TABLE IF NOT EXISTS integrity_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_path TEXT NOT NULL UNIQUE,
    sha256 TEXT NOT NULL,
    file_size INTEGER NOT NULL DEFAULT 0,
    last_verified DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    is_valid INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_integrity_last_verified ON integrity_records(last_verified);
```

### 2.7 recycle_bin — 回收站

创建于迁移 `002_recycle_bin`。

```sql
CREATE TABLE IF NOT EXISTS recycle_bin (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    file_id INTEGER NOT NULL,
    original_path TEXT NOT NULL,
    original_name TEXT NOT NULL,
    trash_path TEXT NOT NULL,
    file_type TEXT NOT NULL DEFAULT 'file',
    file_size INTEGER NOT NULL DEFAULT 0,
    is_dir INTEGER NOT NULL DEFAULT 0,
    deleted_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expire_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_recycle_bin_user_id ON recycle_bin(user_id);
CREATE INDEX IF NOT EXISTS idx_recycle_bin_expire_at ON recycle_bin(expire_at);
```

## 3. 计划中（尚未实现）

以下表为未来功能预留，当前代码库中尚未创建对应的迁移。

### 3.1 file_meta — 文件元数据缓存（搜索/缩略图用）

计划用于存储文件内容哈希（xxHash 快速扫描、SHA256 深度扫描）、MIME 类型、缩略图路径、提取的文本内容等。服务于未来的全文搜索和缩略图缓存功能。

```sql
-- 计划中，尚未实现
CREATE TABLE file_meta (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    file_id         INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    file_hash       TEXT,                     -- SHA256 哈希（深度扫描用）
    fast_hash       TEXT,                     -- xxHash（快速扫描用）
    content_type    TEXT,                     -- 'document' | 'image' | 'video' | 'audio' | 'archive' | 'other'
    thumbnail_path  TEXT,                     -- 缩略图相对路径
    text_content    TEXT,                     -- 文档文本内容（搜索用）
    last_checked    DATETIME,                 -- 最后完整性检查时间
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### 3.2 search_index_meta — 搜索索引元数据

计划用于存储全文搜索索引的版本号、最后索引时间等元信息。

```sql
-- 计划中，尚未实现
CREATE TABLE search_index_meta (
    id    INTEGER PRIMARY KEY AUTOINCREMENT,
    key   TEXT NOT NULL UNIQUE,
    value TEXT NOT NULL
);
```

## 4. schema 版本管理

| 文件 | 创建的表 | 状态 |
|---|---|---|
| `internal/database/migrations/001_init.sql` | users, files, shares, permissions, audit_logs, integrity_records | 已实现 |
| `internal/database/migrations/002_recycle_bin.sql` | recycle_bin | 已实现 |
| （未来迁移） | file_meta | 计划中 |
| （未来迁移） | search_index_meta | 计划中 |

## 5. 索引策略

- 频繁查询的字段均建立索引：用户 ID、文件路径、文件名、分享 token、时间戳
- `files.is_pinned` 和 `files.is_favorite` 建有索引，支持高效查询置顶/收藏文件
- SQLite 使用 WAL 模式提升并发读性能
- 大表（audit_logs、file_meta）按时间分表策略（后续迭代）

## 6. 审计关联说明

所有修改性操作（文件增删改、分享创建、权限变更、登录）均写入 `audit_logs` 表。写操作与业务操作在同一事务中完成，确保审计不丢失。
