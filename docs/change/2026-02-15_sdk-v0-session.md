# 2026-02-15 - SDK v0：Session/Transport 基础（PR2-SDK-1）

## 变更背景 / 目标
为推进“彻底重构（Core/Server/子协议解耦、Win 上移为应用层）”，需要将客户端侧重复实现的底层能力抽出为统一 SDK，避免 Win/CLI 等各自维护连接、HeaderTcp v2 编解码、trace_id/hop_limit 默认值、以及 `{action,data}` JSON 封装逻辑，导致行为漂移与维护成本增加。

本次变更目标（SDK v0，最小可用）：
1) 建立独立仓库 `MyFlowHub-SDK`（Go module：`github.com/yttydcs/myflowhub-sdk`）。
2) 提供 `session`（connect/close/send/readloop）与 `transport`（envelope 编解码）两个基础包。
3) 保持与现有 Win 客户端最小实现一致：Send 时补齐 `trace_id` 与 `hop_limit`（默认 16）。

## 具体变更内容
### 新增（仓库骨架）
- `go.mod`：module/Go/toolchain 定义；声明依赖 `myflowhub-core`、`myflowhub-proto`
- `README.md`：仓库定位、依赖方向与版本演进计划
- `.gitignore`：Go/IDE 常规忽略项
- `plan.md`：本 workflow 的需求/架构/计划与验收

### 新增（功能包）
- `transport/message.go`
  - `Message{Action, Data(json.RawMessage)}`：用于 DecodeMessage 的承载
  - `EncodeMessage(action, data)`：输入校验（action 必填）+ JSON 编码
  - `DecodeMessage(payload)`：返回 `action + raw data`，避免二次反序列化
- `session/session.go`
  - `Session`：`Connect/Close/Send` + `readLoop`
  - `Send` 自动补齐：
    - `hop_limit`：`header.DefaultHopLimit`（16）
    - `trace_id`：随机种子 + 递增序列（0 视为未设置）
  - `Close` 后读循环退出时 **不再回调 onError**（避免“正常关闭也报错”的噪音）

### 测试
- `transport/message_test.go`：输入校验 + roundtrip
- `session/session_test.go`：net.Pipe 下验证 Send 默认字段补齐、readLoop 分发

## plan.md 任务映射
- S1 - 初始化仓库骨架 ✅
- S2 - 新增 transport 包（envelope 编解码）✅
- S3 - 新增 session 包（connect/send/readloop）✅
- S4 - 全量回归与格式化 ✅（`go test ./... -count=1 -p 1` 通过）
- S5 - Code Review + 归档 ✅

## 关键设计决策与权衡
- **v0 最小化**：本次只做 Session/Transport，不引入 Broker/Awaiter，避免一次性设计过大；等待语义留到 SDK v1 独立 PR。
- **Decode 使用 RawMessage**：解包后只解析 action + raw data，避免框架层做多余反序列化，提升性能并把类型选择留给调用方。
- **Close 不触发 onError**：把“用户主动关闭”与“异常断链”区分，减少上层误报；异常断链仍会回调 onError。
- **依赖方向严格**：SDK 仅依赖 Core/Proto 与标准库，不依赖 Server/Win，保证可复用性与稳定边界。

## Code Review（结论：通过）
- 需求覆盖：通过（SDK v0 仓库落地，Session/Transport 能力齐全）
- 架构合理性：通过（依赖方向清晰；包边界最小；为 v1/v2 预留扩展点）
- 性能风险：通过（DecodeMessage 使用 RawMessage；读循环 bufio.Reader；无多余 I/O）
- 可读性与一致性：通过（命名与 Win 现有实现一致；错误信息明确）
- 可扩展性与配置化：通过（v0 保持简单；后续可在不破坏 API 的前提下扩展 Options/Broker）
- 稳定性与安全：通过（输入校验完善；未连接发送有明确错误；Close 不误报）
- 测试覆盖情况：通过（关键路径与边界均有单测）

## 测试与验证方式 / 结果
- `GOTMPDIR=d:\\project\\MyFlowHub3\\.tmp\\gotmp`
- `go test ./... -count=1 -p 1`（通过）

## 潜在影响与回滚方案
### 潜在影响
- 本仓库当前处于 v0 阶段，API 仍可能演进；建议在 Win 接入前先以小步 PR 逐处迁移并保留回滚能力。
- 开发期 `go.mod` 可能包含 `replace` 用于本地联调；后续发布版本时应移除并改为 tag 依赖。

### 回滚方案
- revert 本仓库本次提交（或按提交粒度回滚 S1/S2/S3）。

