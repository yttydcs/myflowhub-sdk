# Todo - SDK 下游依赖对齐 Core v0.4.4

## 目标与状态
- 目标：将 `github.com/yttydcs/myflowhub-core` 依赖从 `v0.4.3` 升级到 `v0.4.4`，为 Win 下游提供稳定版本基线。
- 当前状态：SDK `main` 位于 `v0.1.5`，Core 依赖为 `v0.4.3`。

## 任务清单
- [ ] SDK-1 更新 `go.mod` 的 Core 版本到 `v0.4.4`
- [ ] SDK-2 运行测试验证（`go test ./... -count=1`）
- [ ] SDK-3 更新变更归档 `docs/change/2026-03-15_bump-core-v0.4.4-sdk.md`
- [ ] SDK-4 提交、合并、打 tag（`v0.1.6`）

## 验收条件
- `go.mod` 版本变更准确，代码无行为改动。
- 测试通过。
- 归档包含背景、改动、验证、回滚。

## 回滚点
- 回退 `go.mod` 与归档文档。
