# PureFS 存储层接口规范

## 1. StorageDriver 接口

所有存储后端必须实现统一的 `StorageDriver` 接口，上层 `fs` 模块无感知切换存储后端。

```go
package storage

import (
    "context"
    "io"
    "time"
)

// FileInfo 文件元数据
type FileInfo struct {
    Path       string
    Name       string
    IsDir      bool
    Size       int64
    ModTime    time.Time
    MimeType   string
    ETag       string
}

// StorageDriver 统一存储驱动接口
type StorageDriver interface {
    // Read 读取文件内容
    Read(ctx context.Context, path string) (io.ReadCloser, error)

    // Write 写入文件（不存在则创建，存在则覆盖）
    Write(ctx context.Context, path string, reader io.Reader) error

    // Delete 删除文件或目录（递归删除）
    Delete(ctx context.Context, path string) error

    // Stat 获取文件/目录元信息
    Stat(ctx context.Context, path string) (*FileInfo, error)

    // List 列出目录下所有项目（非递归）
    List(ctx context.Context, dirPath string) ([]*FileInfo, error)

    // Copy 复制文件（仅文件，不跨后端）
    Copy(ctx context.Context, srcPath, dstPath string) error

    // Move 移动/重命名文件或目录
    Move(ctx context.Context, srcPath, dstPath string) error

    // Mkdir 创建目录
    Mkdir(ctx context.Context, path string) error

    // Checksum 计算文件哈希
    Checksum(ctx context.Context, path string, algorithm ChecksumAlgorithm) (string, error)
}

// ChecksumAlgorithm 哈希算法
type ChecksumAlgorithm string

const (
    ChecksumXXHash ChecksumAlgorithm = "xxhash"  // 快速扫描
    ChecksumSHA256 ChecksumAlgorithm = "sha256"  // 深度校验
)
```

## 2. 内置实现

### 2.1 LocalDriver — 本地磁盘驱动

- 文件直接存储在操作系统文件夹中
- 路径映射：`/data/storage/<path>` → 真实文件系统路径
- 权限继承文件系统权限

### 2.2 S3Driver — S3 兼容对象存储

- 支持任何 S3 兼容 API（MinIO、AWS S3、Cloudflare R2、Backblaze B2…）
- 配置项：endpoint、region、bucket、access_key、secret_key、path_style

## 3. 存储后端配置

```yaml
storage:
  driver: local  # local | s3
  local:
    path: /data/purefs/storage
  s3:
    endpoint: https://s3.amazonaws.com
    region: us-east-1
    bucket: purefs-data
    access_key: ""
    secret_key: ""
    path_style: virtual  # virtual | path
```

## 4. 扩展规范

新增存储后端需：

1. 实现 `StorageDriver` 接口的全部方法
2. 在 `storage/` 下创建新包
3. 在 `config` 中注册驱动名称到工厂函数的映射
