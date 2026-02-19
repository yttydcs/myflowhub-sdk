# 2026-02-17 - SDK Awaiter：增加 onFrame hook（匹配帧也可观察）（PR13-SDK-Await-Hooks）

## 变更背景 / 目标
SDK v1 已提供 `await.Client.SendAndAwait`（按 `MsgID + SubProto + Action` 等待响应）。但 `await.Client` 的既有行为是：
- **匹配成功的响应帧会被拦截并 deliver**（不再走 `onUnmatched`），以避免重复消费。

这会导致一个实际接入问题：当上层（例如 Win）需要“无论是否匹配成功，都要把每一帧发布到事件总线/日志/前端”时，匹配成功帧会“不可见”。

本变更的目标：
1) 给 `await.Client` 增加一个向后兼容的 **全帧观察 hook**（`onFrame`）。
2) 保持 Awaiter 语义不变：**匹配成功仍不会走 `onUnmatched`**。
3) 为 Win 等调用方提供“send+await 与 session.frame 事件并存”的基础能力。

## 具体变更内容
### 新增
- `await/client.go`
  - 新增 `Client.SetOnFrame(func(core.IHeader, []byte))`
  - 在 `handleFrame` 开始处回调 `onFrame`（若设置），随后按原逻辑 deliver / onUnmatched

### 测试
- `await/client_test.go`
  - 覆盖：匹配成功的响应帧也会触发 `onFrame`（同时保持不触发 `onUnmatched`）

### 文档
- `plan.md`：本 workflow 的需求/架构/计划与验收
- `docs/plan_archive/plan_archive_2026-02-16_sdk-v1-await.md`：归档上一轮计划文档（便于审计回放）

## plan.md 任务映射
- SAH1 - SDK：await.Client 增加 onFrame hook ✅
- SAH2 - SDK：补齐单测 ✅
- SAH3 - 回归测试 ✅（`go test ./... -count=1 -p 1`）
- SAH4 - Code Review + 归档 ✅

## 关键设计决策与权衡
- **不改 `NewClient` 签名**：通过 `SetOnFrame` 增量扩展，避免破坏现有调用方。
- **onFrame 先于 deliver**：保证调用方能观察到 matched frame；同时仍保持 deliver 的“拦截”语义。
- **性能与调用约束**：onFrame 运行于 readLoop 回调线程；SDK 不做节流/异步，调用方需保证回调轻量不阻塞。

## Code Review（结论：通过）
- 需求覆盖：通过（onFrame 可观察 matched frame；不影响 onUnmatched 语义）
- 架构合理性：通过（增量 hook；不破坏 await/broker/client 边界）
- 性能风险：通过（仅增加一次可选回调；不增加额外 JSON 解码）
- 可读性与一致性：通过（命名清晰；注释说明线程模型）
- 可扩展性与配置化：通过（未来可扩展为 options/多 hook，不破坏现有 API）
- 稳定性与安全：通过（默认不启用；不改变 close/错误路径）
- 测试覆盖情况：通过（新增断言覆盖 matched frame 也触发 onFrame）

## 测试与验证方式 / 结果
```powershell
$env:GOTMPDIR='d:\\project\\MyFlowHub3\\.tmp\\gotmp'
New-Item -ItemType Directory -Force -Path $env:GOTMPDIR | Out-Null
go test ./... -count=1 -p 1
```
结果：通过（Windows）。

## 潜在影响与回滚方案
### 潜在影响
- 若调用方设置了 onFrame 且回调较重，可能拖慢 readLoop（属于使用方式风险）。

### 回滚方案
- `git revert f0b47ff`（移除 onFrame hook 与对应测试）
- 如需同时回滚本 workflow 文档：再 revert `1336a48`

