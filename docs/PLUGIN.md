# PureFS 插件接口规范

## 1. 总体设计

### 1.1 阶段规划

| 阶段 | 加载方式 | 说明 |
|------|----------|------|
| 一期 | 编译期注册 | 插件代码在编译时集成，通过接口注册 |
| 二期 | gRPC 子进程 | 插件作为独立进程，通过 gRPC 通信，支持热插拔 |

### 1.2 插件分类

- **存储驱动插件**：实现新的存储后端
- **功能扩展插件**：OCR、相册整理、在线编辑、离线下载等

## 2. 插件接口定义

### 2.1 插件生命周期

```go
type Plugin interface {
    // Info 返回插件元信息
    Info() PluginInfo

    // Init 初始化插件，在系统启动时调用
    Init(ctx context.Context, deps *PluginDeps) error

    // Shutdown 关闭插件，在系统关闭时调用
    Shutdown(ctx context.Context) error
}

type PluginInfo struct {
    ID          string   // 唯一标识，如 "purefs.ocr"
    Name        string   // 显示名称
    Version     string   // 语义化版本
    Author      string
    Description string
    Category    PluginCategory // storage | extension
}

type PluginDeps struct {
    DB      *sql.DB
    Storage storage.StorageDriver
    Config  *config.Config
    Logger  Logger
}
```

### 2.2 功能扩展钩子

```go
// FileHook 文件操作钩子
type FileHook interface {
    // OnFileCreated 文件创建后触发
    OnFileCreated(ctx context.Context, path string) error

    // OnFileDeleted 文件删除后触发
    OnFileDeleted(ctx context.Context, path string) error

    // OnFileMoved 文件移动/重命名后触发
    OnFileMoved(ctx context.Context, srcPath, dstPath string) error
}

// SearchIndexHook 搜索索引钩子
type SearchIndexHook interface {
    // OnIndexFile 文件需要建立索引时触发
    OnIndexFile(ctx context.Context, path string, content io.Reader) (text string, err error)
}
```

### 2.3 存储驱动插件

```go
type StoragePlugin interface {
    Plugin
    CreateDriver(ctx context.Context, config map[string]any) (storage.StorageDriver, error)
}
```

## 3. 插件目录结构

```
plugin/
├── internal/           ← 官方内置插件（随主程序编译）
│   ├── ocr/           ← OCR 文本识别
│   ├── gallery/       ← 相册智能整理
│   └── offline-dl/    ← 离线下载
│
└── external/          ← 第三方插件（二期 gRPC 模式）
```

## 4. 插件配置

```yaml
plugins:
  enabled:
    - purefs.ocr
    - purefs.gallery
  config:
    purefs.ocr:
      engine: tesseract   # tesseract | paddle
      language: chi_sim+eng
    purefs.gallery:
      auto_album: true
```
