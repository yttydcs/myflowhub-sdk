# Plan - SDK await.Client 增加 onFrame hook（PR13-SDK-Await-Hooks）

## Workflow 信息
- Repo：`MyFlowHub-SDK`
- 分支：`feat/sdk-await-hooks`
- Worktree：`d:\project\MyFlowHub3\worktrees\pr13-win-auth-await\MyFlowHub-SDK`
- Base：`main`
- 参考：
  - `d:\project\MyFlowHub3\target.md`
  - `d:\project\MyFlowHub3\repos.md`
  - `d:\project\MyFlowHub3\guide.md`（commit 信息中文）

## 约束（边界）
- 仅改 `await.Client`：新增“接收帧 tap”能力（onFrame）。
- **保持向后兼容**：
  - `await.NewClient(ctx, onUnmatched, onError)` 签名不变。
  - 若调用方不设置 onFrame，则行为与当前一致。
- 不改 `session`/`transport` 的既有行为（除非测试需要的最小调整）。
- 必须补齐单测覆盖 onFrame 语义。

---

## 1) 需求分析

### 目标
- 让调用方（Win）在使用 `await.Client.SendAndAwait` 时：
  - 匹配成功的响应帧仍然可以被“观察/记录/转发”（例如发布 `session.frame` 事件、写日志）。
  - 但不改变 Awaiter 的既有语义：**匹配成功帧不走 onUnmatched**。

### 范围（必须 / 不做）
#### 必须（本 PR）
- 新增 onFrame hook（可选）：收到任何帧时都会回调（包括将被 deliver 的帧）。
- 暴露设置方式：`SetOnFrame(func(core.IHeader, []byte))`。
- 单测：断言 matched response 也会触发 onFrame（同时 unmatched 不触发）。

#### 不做（本 PR）
- 不改变匹配规则（仍是 `MsgID+SubProto+Action`）。
- 不改变 `Broker` 的行为。
- 不引入新的公共依赖。

### 验收标准
- `go test ./... -count=1 -p 1` 通过（Windows）。
- 新增/更新单测覆盖 onFrame 行为。

### 风险
- onFrame 运行于 readLoop 回调线程，调用方需确保回调轻量；本 PR 仅提供机制，不在 SDK 内做节流。

---

## 2) 架构设计（分析）

### 总体方案（采用）
- 在 `await.Client` 增加一个可选的 `onFrame` 回调：
  - `handleFrame` 开始处先调用 onFrame（若设置）。
  - 随后按既有逻辑判断并 deliver / 或 onUnmatched。
- 提供 `SetOnFrame` 方法，避免破坏现有 `NewClient` 的参数签名。

### 备选对比（为什么不选）
- 备选 A：改为 deliver 后也调用 onUnmatched
  - 问题：改变语义，可能导致调用方重复消费同一帧。
- 备选 B：让 Win 自己实现 broker+deliver（不改 SDK）
  - 问题：等待语义在 Win 分叉，未来难统一；且会复制 SDK 内部优化（HasMsgSub 快路径）。

---

## 3.1) 计划拆分（Checklist）

## 问题清单（阻塞：否）
- 无（本 PR 为向后兼容的增量 hook）。

### SAH1 - SDK：await.Client 增加 onFrame hook
- 目标：提供“全帧观察”能力，保证 matched response 也能被上层记录/分发。
- 涉及文件：
  - `await/client.go`
- 验收条件：
  - onFrame 可选；未设置时行为不变。
- 回滚点：
  - revert 本提交（回到旧行为）。

### SAH2 - SDK：补齐单测
- 目标：锁定语义，避免未来回归。
- 涉及文件：
  - `await/client_test.go`
- 验收条件：
  - matched response 触发 onFrame。
  - 匹配成功不触发 onUnmatched（保持既有语义）。
- 回滚点：
  - revert 测试提交。

### SAH3 - 回归测试
- 命令：
  - `$env:GOTMPDIR='d:\\project\\MyFlowHub3\\.tmp\\gotmp'`
  - `New-Item -ItemType Directory -Force -Path $env:GOTMPDIR | Out-Null`
  - `go test ./... -count=1 -p 1`
- 验收条件：通过。

### SAH4 - Code Review（阶段 3.3）+ 归档变更（阶段 4）
- 归档文件：
  - `docs/change/2026-02-17_sdk-await-onframe.md`
- 验收条件：
  - Review 覆盖：需求/架构/性能/安全/测试；
  - 归档包含：任务映射、关键决策、测试命令与回滚方案。

