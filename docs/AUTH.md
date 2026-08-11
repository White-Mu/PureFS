# PureFS 认证与安全设计

## 1. 认证机制

### 1.1 方案选择

- **JWT（无状态 Token）**
- Token 有效期：24 小时（可配置）
- 支持 Refresh Token 机制（有效期 30 天）

### 1.2 JWT 结构

```json
{
  "sub": "user_id",
  "username": "admin",
  "role": "admin",
  "iat": 1680000000,
  "exp": 1680086400
}
```

使用 HMAC-SHA256 或 RSA 签名，密钥从环境变量或配置文件读取。

### 1.3 接口防护

- 除 `/auth/login`、`/auth/register`、`/s/:share_id` 外，所有 API 需携带 JWT
- JWT 从 `Authorization: Bearer <token>` 头提取
- 中间件校验 Token 有效性，注入用户信息到 Context

## 2. 双因素认证 (2FA)

- 基于 TOTP（RFC 6238）标准
- 兼容 Google Authenticator、Authy、Microsoft Authenticator 等
- 用户可在个人设置中启用/禁用
- 启用后在 `/auth/login` 时额外校验 6 位 TOTP 码

## 3. 密码策略

- 使用 bcrypt 哈希（cost=12）
- 最少 8 位字符
- 支持管理员设置密码复杂度要求（后续迭代）

## 4. 服务端透明加密 (At-Rest)

### 4.1 设计原则

- 文件在存储层写入磁盘前加密，读取时解密
- 加解密对上层 `fs` 模块完全透明
- 密钥由用户控制，支持密钥轮换

### 4.2 加密方案

- 算法：AES-256-GCM
- 每个文件使用独立随机密钥（DEK，Data Encryption Key）
- DEK 使用主密钥（KEK，Key Encryption Key）加密后存储在数据库或配置中
- 加解密发生在 `StorageDriver` 的 Read/Write 方法中（可选装饰器模式）

```
用户配置主密钥 (KEK)
       │
       ▼
  生成随机 DEK → 用 KEK 加密 DEK → 存储加密后的 DEK
       │
       ▼
  用 DEK 加密文件数据 → 写入磁盘
```

### 4.3 密钥管理

- KEK 来源：配置文件、环境变量、KMS 服务（后续迭代）
- 支持密钥轮换：旧文件使用旧 DEK 保留，新文件使用新 KEK 加密的 DEK

## 4.5 端到端加密 (E2EE)

E2EE 与第 4 节的服务端透明加密是**两套独立可选的加密层**：
- **服务端透明加密**：在存储层加密，对客户端透明，服务器持有密钥
- **E2EE（客户端加密）**：在浏览器内加密，服务器只存密文，**无法解密**

### 4.5.1 加密模型（信封加密）

```
E2EE 口令 + 随机盐
       │ PBKDF2-SHA256（15 万次迭代）
       ▼
   KEK（密钥加密密钥）
       │ AES-256-GCM
       ▼
   随机 32 字节主密钥（客户端生成）──→ 包装后存服务器 users.e2ee_wrapped_key
       │ AES-256-GCM
       ▼
   每个文件独立 DEK（数据加密密钥）──→ 包装后存服务器 files.dek_ciphertext
       │ AES-256-GCM
       ▼
   文件明文 ──→ 密文（nonce + ciphertext + tag），服务器仅存此密文
```

### 4.5.2 安全属性

- 服务器永远看不到文件明文，也看不到主密钥（只能拿到包装后的密文）
- **忘记口令 = 数据永久不可恢复**。备份密钥（即包装后的主密钥）可以在忘记口令时重新解锁
- 主密钥只存内存（Zustand store），不写 localStorage、不发往服务器
- E2EE 文件不参与全文索引（密文无法提取文本），名称仍可被服务器看到
- 复制文件时保留包装 DEK，副本可正常解密

### 4.5.3 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/users/e2ee/status` | 查询是否启用 + 获取 salt / 包装主密钥 |
| POST | `/api/users/e2ee` | 上传 salt + 包装主密钥（启用） |
| DELETE | `/api/users/e2ee` | 清除密钥（关闭，已有密文不可再解密） |

### 4.5.4 上传格式

上传 E2EE 文件时，multipart 表单额外携带：

```
is_e2ee=true
dek_ciphertext=<base64(nonce(12) + AES-GCM(DEK, 主密钥))>
kek_version=0
```

`kek_version=0` 标记"客户端包装的 DEK"，与服务器端加密（`kek_version>=1`）区分。

## 5. 操作审计

- 所有敏感操作记录到 `audit_log` 表
- 记录字段：操作用户、动作类型、目标路径、IP 地址、User-Agent、时间戳
- 审计日志不可篡改（仅追加）

## 6. 安全防护

- CORS 限制（仅允许配置的前端域名）
- 速率限制（/auth/login 5 次/分钟）
- 上传文件类型/大小限制（可配置）
- SQL 注入防护（使用参数化查询）
- XSS 防护（Content-Security-Policy 头）
- CSRF 防护（SameSite Cookie + JWT）
