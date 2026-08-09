# 代理配置

所有 GitHub 相关操作可通过以下代理加速：

```powershell
# Go
$env:GOPROXY="https://goproxy.cn,direct"
$env:GONOSUMCHECK="*"
$env:GONOSUMDB="*"

# npm (如后续需要)
npm config set registry https://registry.npmmirror.com
```

Git 也可以配置使用该代理（如后续 clone 子模块）：
```powershell
git config --global http.proxy https://gh.jasonzeng.dev
```

# Go 编译器内存限制（Go 1.26+ 在 Windows 上可能吃内存较多）

```powershell
$env:GOMEMLIMIT="2097152000"   # 限制 Go 进程内存上限 2GB
$env:GOGC="50"                 # 更早触发 GC 减少峰值内存

## Agent skills

### Issue tracker

Issues live as GitHub issues. See `docs/agents/issue-tracker.md`.

### Triage labels

The five canonical triage labels are used. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context layout. See `docs/agents/domain.md`.
```
