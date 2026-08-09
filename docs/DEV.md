# PureFS 开发规范

## 1. 代码风格

### Go

- 使用 `go vet` 和 `golangci-lint` 进行代码检查
- 遵循 [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- 错误处理：必须检查所有返回 error 的调用，不使用 `_` 忽略
- 包名小写单数，不含下划线
- 接口名以 `er` 结尾（如 `StorageDriver`、`FileHook`）

### TypeScript / React

- 使用 TypeScript 严格模式
- 组件使用函数组件 + Hooks
- 文件名使用 camelCase（组件使用 PascalCase）
- CSS 使用 CSS Modules 或 Tailwind CSS（待定）
- 状态管理使用 React Context + useReducer（暂不引入 Redux）

## 2. 提交规范

使用 [Conventional Commits](https://www.conventionalcommits.org/)：

```
<type>(<scope>): <description>

feat:    新功能
fix:     修复 bug
docs:    文档变更
style:   代码格式（不影响功能）
refactor:重构
test:    测试
chore:   构建/工具变更
```

示例：
```
feat(storage): implement LocalDriver CRUD methods
fix(auth): handle expired JWT token gracefully
docs(api): add search endpoint documentation
```

## 3. 分支策略

```
main          ← 稳定版本
├── dev       ← 开发主分支
│   ├── feat/xxx    ← 功能分支
│   ├── fix/xxx     ← 修复分支
│   └── refactor/xxx ← 重构分支
```

- 功能开发从 `dev` 切出 `feat/xxx` 分支
- 完成后通过 PR/Merge Request 合并回 `dev`
- `main` 仅从 `dev` release 合并

## 4. 测试要求

- 所有核心逻辑（存储驱动、认证、文件操作）必须编写单元测试
- API 端点编写集成测试
- 测试覆盖率目标：核心模块 ≥ 80%

### 运行测试

```bash
# Go 后端
cd PureFS-App
go test ./internal/...
go test ./pkg/...

# Web 前端
cd PureFS-Web
npm run test
npm run lint
```

## 5. 构建方式

```bash
# Go 后端
cd PureFS-App
go build -o purefsd ./cmd/purefsd/
go build -o purefsd ./cmd/purefsd/  # Linux
GOOS=linux GOARCH=arm64 go build -o purefsd-arm64 ./cmd/purefsd/  # 树莓派

# 前端
cd PureFS-Web
npm run build
```
