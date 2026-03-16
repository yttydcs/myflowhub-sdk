# 变更背景 / 目标

在 QUIC endpoint 功能已接入后，将 SDK 对 Core 的依赖从开发态 pseudo-version 收敛到正式 tag，便于下游稳定引用与发布。

# 具体变更内容

## 修改
- `go.mod`
  - `github.com/yttydcs/myflowhub-core`：
    - `v0.4.7-0.20260316021423-d992975ec6ad`
    - -> `v0.4.7`
- `go.sum`
  - 同步模块校验项，移除/更新 pseudo-version 对应记录。

# 对应任务映射

- `QUIC-REL-1`：下游版本对齐与发布

# 关键设计决策与权衡

- 使用正式 tag 替换 pseudo-version：
  - 优点：版本稳定、可审计、便于跨仓依赖追踪；
  - 代价：需要按 Core -> SDK 顺序发布，确保 tag 已可解析。

# 测试与验证方式 / 结果

- 执行：
  - `GOWORK=off go mod tidy`
  - `GOWORK=off go test ./...`
- 结果：
  - 全量测试通过。

# 潜在影响与回滚方案

- 潜在影响：
  - 无功能行为变更，仅依赖来源从 commit 固定到 tag。
- 回滚方案：
  - 回退本次提交并恢复旧依赖版本；重新执行测试确认。
