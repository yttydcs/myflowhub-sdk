# Plan - SDK：支持 `bt+rfcomm://`（Bluetooth Classic RFCOMM）Dial（对齐 TCP 能力）

## Workflow 信息
- Repo：`MyFlowHub-SDK`
- 分支：`feat/bluetooth-rfcomm-transport`
- Worktree：`d:\project\MyFlowHub3\worktrees\feat-bluetooth-rfcomm-transport\repo\MyFlowHub-SDK`
- Base：`main`
- 依赖仓：
  - `MyFlowHub-Core`：RFCOMM dial/endpoint（本分支新增）

## 背景 / 问题陈述（事实，可审计）
- SDK 当前 `Session.Connect(addr string)` 仅支持 TCP（`net.Dial("tcp")` + `net.Conn`）。
- 需求：除 TCP 外，SDK 侧也应具备与 TCP 同等的“连接/收发帧”能力，使客户端可通过 RFCOMM 接入同一网络。

## 目标
1) 在不牺牲可维护性的前提下，让 SDK 支持通过 endpoint 进行连接：
  - `tcp://host:port`（以及现有 `host:port` 兼容）
  - `bt+rfcomm://<bdaddr>?uuid=...&channel=...&secure=...`
2) 连接底座从 `net.Conn` 泛化为 `io.ReadWriteCloser`（或 `core.IPipe`），以适配非 TCP 承载。
3) 保持上层 HeaderCodec/帧语义不变：仍使用 `HeaderTcpCodec` 对字节流进行 encode/decode。

## 非目标
- 不实现“按设备名扫描/解析到 MAC”（由 transport 扩展点与平台实现负责）。
- 不引入 payload 业务解析。

## 验收标准
- `go test ./...` 通过（workflow-local go.work 下可联动 Core）。
- 新增单测覆盖：
  - endpoint 解析：tcp/bt+rfcomm、缺失参数、非法 uuid/channel/bdaddr 等。
  - Session 关闭/重连/并发 Send 的基本边界。
- 若引入 breaking change（如公开 API 签名调整），必须在 `docs/change` 中明确标注并给迁移示例。

## 3.1) 计划拆分（Checklist）

### SDK-BT0 - 归档旧 plan（已执行）
- 已执行：`git mv plan.md docs/plan_archive/plan_archive_2026-03-12_bluetooth-rfcomm-transport-sdk-prev.md`

### SDK-BT1 - 依赖升级：对齐 Core（含 RFCOMM 能力）
- 目标：升级 `go.mod` 的 `myflowhub-core` 到包含 RFCOMM 的版本；开发期通过 workflow-local go.work 联动，最终确保 `GOWORK=off` 可用。
- 涉及文件：`go.mod`、`go.sum`
- 验收条件：`GOWORK=off go test ./...` 可编译（需要 Core 发布版本后执行）。
- 回滚点：revert。

### SDK-BT2 - 引入 endpoint connect（tcp + bt+rfcomm）
- 目标：新增 `ConnectEndpoint(endpoint string)`（或等价）并保持现有 `Connect(addr string)` 行为不回归。
- 涉及文件（预期）：
  - `session/session.go`
  - `session/endpoint*.go`（如拆分）
- 验收条件：
  - tcp 路径与现有一致；
  - bt+rfcomm 能进入 dial 路径并在无真实环境下返回可读错误（由 Core 实现提供）。
- 回滚点：revert。

### SDK-BT3 - Session 底座抽象（io.ReadWriteCloser / Pipe）
- 目标：内部不再强依赖 `net.Conn`，读写循环使用 `io.Reader/io.Writer`。
- 性能注意：
  - 避免每帧重复分配；必要时复用 bufio.Reader。
- 验收条件：`go test ./...` 通过；Close 不泄漏 goroutine。
- 回滚点：revert。

### SDK-BT4 - Code Review（强制）
- 审查项：需求覆盖/架构/性能/可读性/扩展性/稳定性与安全/测试覆盖。

### SDK-BT5 - 归档变更（强制）
- 输出：`docs/change/2026-03-12_bluetooth-rfcomm-transport-sdk.md`
- 标注：重大变更（SDK transport 能力扩展；可能涉及连接抽象变更）。

### SDK-BT6 - 合并 / push（需 workflow 结束后执行）
- 在 `repo/MyFlowHub-SDK` 合并到 `main` 并 push。

