# Plan - SDK：Awaiter 支持 File CTRL 帧解析（KindCtrl + JSON）（PR18-SDK-Await-FileCtrl）

## Workflow 信息
- Repo：`MyFlowHub-SDK`
- 分支：`feat/await-file-ctrl`
- Worktree：`d:\project\MyFlowHub3\worktrees\pr18-file-ctrl-await\MyFlowHub-SDK`
- Base：`main`
- 参考：
  - `d:\project\MyFlowHub3\target.md`
  - `d:\project\MyFlowHub3\repos.md`
  - `d:\project\MyFlowHub3\guide.md`（commit 信息中文）
- 依赖（本地 replace/junction）：
  - `..\MyFlowHub-Core` / `..\MyFlowHub-Proto`

## 约束（边界）
- 仅改 `await.Client` 的响应帧解析逻辑：
  - `SubProto=file` 且 `payload[0]==KindCtrl` 时，对 `payload[1:]` 解包 `action+data`；
  - 其余子协议保持现状（对整段 payload 解包）。
- 匹配规则不变：`MsgID + SubProto + Action`。
- 行为不变：
  - matched frame 仍会 deliver（不会走 onUnmatched）；
  - `SetOnFrame` 仍能观察 matched frame（Win 依赖该语义发布 `session.frame`）。
- 不改 `session`/`transport` 的既有行为（除测试所需的最小调整）。

## 当前状态（事实，可审计）
- SDK v1 Awaiter 当前仅支持“payload 直接为 JSON(message)”的子协议。
- File 子协议的 CTRL payload 为：`KindCtrl(0x01) + JSON(action+data)`：
  - 现状：Awaiter 对整段 payload 做 JSON 解包会失败，导致无法 deliver File CTRL 的 `*_resp`。
- 本轮 workflow 已确认：Server 会补齐 `read_resp/write_resp` 继承请求 `MsgID/TraceID`；
  - 本 PR 负责 SDK 侧“正确解包 File CTRL”以完成 send+await 闭环。

---

## 1) 需求分析

### 目标
1) 让 `await.Client` 能够匹配并 deliver File CTRL 响应帧（`read_resp/write_resp`），以支持上层（Win）对 File `list/read_text` 的 send+await。
2) 保持 Awaiter 现有语义不变（onFrame/onUnmatched/匹配规则）。

### 验收标准
- 新增单测覆盖：File CTRL（KindCtrl 前缀）响应能被 SendAndAwait 成功匹配返回。
- `go test ./... -count=1 -p 1` 通过。

---

## 2) 架构设计（分析）

### 总体方案（采用）
- 在 `await.Client.handleFrame` 内引入“按 SubProto 选择解包视图”的最小逻辑：
  - 默认：`DecodeMessage(payload)`；
  - File CTRL：`DecodeMessage(payload[1:])`（跳过 kind 字节）。
- 该逻辑只在满足以下条件时执行（保持性能与安全默认）：
  - broker 存在同 `(msg_id, sub_proto)` 的等待者（fast-path gating）；
  - header `Major` 为 `MajorOKResp/MajorErrResp`（保持现状 gating）。

### 备选对比
- 备选 A：在 Win 侧手写匹配（不采用）
  - 会导致等待语义在上层重复实现，且难以处理超时/取消/断线一致性。
- 备选 B：修改 File wire（不采用）
  - 与策略 A（wire 不变）冲突，牵涉面大。

### 错误与安全
- 不改变错误模型：解包失败的帧仍走 onUnmatched（保持既有行为）。

### 性能与测试策略
- 性能：只有在“存在等待者”时才会进入 JSON 解包；并且仅对 File CTRL 多一次切片（`payload[1:]`）。
- 测试：
  - 单测：新增 `SendAndAwait` 场景，服务端回包使用 `KindCtrl + JSON(read_resp)`。

---

## 3.1) 计划拆分（Checklist）

## 问题清单（阻塞：否）
- 已确认：本 PR 与 Server/Win 同步推进，以保证 send+await 闭环。

### AFC1 - 实现：File CTRL payload 解包
- 目标：当 `SubProto=file` 且 `payload[0]==KindCtrl` 时，解包 `payload[1:]` 获取 `action` 用于匹配。
- 涉及文件：
  - `await/client.go`
- 验收条件：
  - File CTRL 的 `*_resp` 能被 deliver；
  - 其他子协议行为不变。
- 回滚点：
  - revert 本提交。

### AFC2 - 单测：覆盖 File CTRL send+await
- 目标：新增单测覆盖 KindCtrl 前缀下的匹配成功。
- 涉及文件：
  - `await/client_test.go`
- 验收条件：
  - `go test ./...` 通过。

### AFC3 - 回归测试
- 命令：
  - `$env:GOTMPDIR='d:\\project\\MyFlowHub3\\.tmp\\gotmp'`
  - `New-Item -ItemType Directory -Force -Path $env:GOTMPDIR | Out-Null`
  - `go test ./... -count=1 -p 1`
- 验收条件：通过。

### AFC4 - Code Review（阶段 3.3）+ 归档变更（阶段 4）
- 归档文件：
  - `docs/change/2026-02-18_sdk-await-file-ctrl.md`
- 验收条件：
  - Review 覆盖：需求/架构/性能/安全/测试；
  - 归档包含：任务映射、关键决策、测试命令与回滚方案。

