# PureFS 搜索模块设计

## 1. 分阶段规划

| 阶段 | 功能 | 技术方案 |
|------|------|----------|
| MVP (一期) | 文件名搜索 + 文档原生文本检索 | bleve 内嵌索引 |
| 二期 | OCR 图片文字识别 | 插件机制 + Tesseract/PaddleOCR |
| 三期 | 可选外部搜索引擎 | Elasticsearch / Meilisearch 适配 |

## 2. 搜索架构

```
用户输入查询词
      │
      ▼
  /api/v1/search
      │
      ▼
  Search Service ──┬──→ bleve 索引（文件名 + 文档文本）
                   │
                   └──→ file_meta 表（降级/扩展查询）
      │
      ▼
  返回 FileInfo 列表
```

## 3. bleve 索引设计

### 3.1 索引映射

```go
type SearchDocument struct {
    Path      string    `json:"path"`
    Name      string    `json:"name"`
    Content   string    `json:"content"`  // 文档文本内容
    Extension string    `json:"extension"`
    FileSize  int64     `json:"file_size"`
    ModTime   time.Time `json:"mod_time"`
}
```

- `Name` 和 `Content` 字段建立全文索引
- `Path` 字段建立关键字索引（支持精确路径查询）
- `Extension`、`FileSize` 支持过滤

### 3.2 索引存储

- 默认存储在 `{data_dir}/search/index.bleve`
- 索引路径、内存上限可配置

## 4. 文档文本提取

- 纯文本文件（.txt/.md/.csv）：直接读取内容
- PDF：使用 extract 库提取文本
- Office 文档（.docx/.pptx/.xlsx）：使用解析库提取文本
- 图片文字：预留钩子，由 OCR 插件处理
- 二进制文件：跳过文本索引

## 5. 索引维护

- 文件上传/修改后异步更新索引（通过任务队列）
- 文件删除后同步从索引移除
- 支持全量重建索引（管理后台操作）
- 搜索索引占用的内存/磁盘可通过配置限制

## 6. OCR 预留接口

```go
// 在 search 包中定义
type OCRExtractor interface {
    // ExtractText 从图片中提取文字
    ExtractText(ctx context.Context, imagePath string) (string, error)
}
```

- MVP 阶段此接口为空实现
- OCR 插件实现此接口后，搜索模块自动调用

## 7. 外部搜索引擎适配（三期）

```go
// SearchEngine 统一接口
type SearchEngine interface {
    Index(ctx context.Context, doc *SearchDocument) error
    Remove(ctx context.Context, path string) error
    Search(ctx context.Context, query string, opts *SearchOpts) ([]*SearchDocument, error)
}

// 实现：
// - BleveEngine（默认）
// - ElasticEngine（可选）
// - MeiliEngine（可选）
```
