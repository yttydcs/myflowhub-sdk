# 2026-02-18 - SDK Awaiter：支持 File CTRL（KindCtrl + JSON）解包（PR18-SDK-Await-FileCtrl）

## 变更背景 / 目标
SDK v1 Awaiter（`await.Client.SendAndAwait`）的匹配维度为：`MsgID + SubProto + Action`。

此前 Awaiter 默认假设“响应 payload 直接为 JSON(message{action,data})”，因此会对整段 payload 做 `DecodeMessage(payload)`。但 File 子协议的 CTRL payload 形态为：

- `KindCtrl(0x01) + JSON(message{action,data})`

这会导致 File CTRL 响应帧（例如 `read_resp/write_resp`）在 SDK 侧解包失败，进而无法 deliver 给等待者，表现为 await 超时。

本次变更目标：
1. 让 Awaiter 能匹配并 deliver File CTRL 响应帧（支持 Win 等上层对 File `list/read_text` 的 send+await）。
2. 保持现有 Awaiter 语义不变：
   - matched frame 仍会 deliver（不会走 `onUnmatched`）
   - `SetOnFrame` 仍能观察 matched frame（Win 依赖该语义发布 `session.frame`）

## 具体变更内容
### 修改
- `await/client.go`
  - 在 `handleFrame` 解包阶段增加“按 SubProto 选择解包视图”的最小逻辑：
    - 默认：`DecodeMessage(payload)`
    - File CTRL：当 `SubProto==file` 且 `payload[0]==KindCtrl` 时，使用 `DecodeMessage(payload[1:])`（跳过 kind 字节）
  - fast-path gating 保持不变：仅当 broker 存在 `(msg_id, sub_proto)` 等待者时才进入 JSON 解包。

### 测试
- `await/client_test.go`
  - 新增用例：服务端回包为 `KindCtrl + JSON(read_resp)` 时，`SendAndAwait(..., expectAction=read_resp)` 能在超时内成功匹配返回。

### 文档
- `plan.md`：本 workflow 的需求/架构/计划与验收
- `docs/plan_archive/plan_archive_2026-02-17_sdk-await-onframe.md`：归档上一轮计划文档（便于审计回放）

## plan.md 任务映射
- AFC1 - 实现：File CTRL payload 解包 ✅
- AFC2 - 单测：覆盖 File CTRL send+await ✅
- AFC3 - 回归测试 ✅
- AFC4 - Code Review + 归档变更 ✅（本文 + Review 结论）

## 关键设计决策与权衡
- **仅对 File 子协议启用 KindCtrl 跳过逻辑**：避免把“kind 前缀”假设泛化到其它子协议，减少误解包风险。
- **保持匹配规则不变**：仍以 `MsgID + SubProto + Action` 作为统一 Awaiter 框架规则，避免上层重复实现等待语义。
- **性能**：仅在存在等待者时增加一次切片（`payload[1:]`）与一次 JSON 解包；复杂度与原逻辑一致。

## Code Review（结论：通过）
- 需求覆盖：通过（File CTRL 响应可被正确解包并 deliver；语义不变）
- 架构合理性：通过（局部解包视图选择；不破坏 await/broker/session 边界）
- 性能风险：通过（fast-path gating 保持；额外开销常量级）
- 可读性与一致性：通过（条件清晰；不引入多分支复杂结构）
- 可扩展性与配置化：通过（未来若有其它 kind 前缀子协议，可按同一模式按需扩展）
- 稳定性与安全：通过（解包失败仍走 onUnmatched；无新权限/网络暴露）
- 测试覆盖情况：通过（新增关键路径单测）

## 测试与验证方式 / 结果
```powershell
$env:GOTMPDIR='d:\\project\\MyFlowHub3\\.tmp\\gotmp'
New-Item -ItemType Directory -Force -Path $env:GOTMPDIR | Out-Null
go test ./... -count=1 -p 1
```
结果：通过（Windows）。

## 潜在影响与回滚方案
### 潜在影响
- File CTRL 响应帧将被 SDK Awaiter 正常 deliver：上层若注册等待者将更容易发现“服务端 action 不匹配 / MsgID 未继承”等问题（表现为超时）。

### 回滚方案
- `git revert 177164b`（撤销 File CTRL 解包逻辑 + 对应单测）
