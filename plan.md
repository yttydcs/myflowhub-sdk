# Plan - SDK：修复客户端 RFCOMM 帧发送的短写问题

## Workflow 信息
- Repo：`MyFlowHub-SDK`
- 分支：`fix/sdk-rfcomm-write-contract`
- Worktree：`d:\project\MyFlowHub3\worktrees\fix-sdk-rfcomm-write-contract\repo\MyFlowHub-SDK`
- Base：`main`
- 依赖仓：
  - `MyFlowHub-Core`：提供统一写出契约与新版本 tag

## 项目目标与当前状态
- 目标：
  - 修复 SDK 客户端在 RFCOMM / 非 TCP 字节流上发送完整帧时未保证写满的问题；
  - 让 SDK session 的发送语义与 Core 一致，避免 client/server 两端行为分叉；
  - 为 Win 客户端 release 提供稳定依赖版本。
- 当前状态：
  - SDK `Session.Send` 直接执行一次 `pipe.Write(frame)`；
  - 对 TCP 往往工作正常，但对 RFCOMM 这类可能短写的流不可靠；
  - 实际表现为 Win 客户端连接成功后，register 帧发出但服务端无法稳定收到完整帧。

## 范围
- 必须：
  - 修复 SDK session 发送完整帧的短写风险；
  - 若合适，复用与 Core 一致的写出辅助能力或等价实现；
  - 补充 SDK 侧短写回归测试；
  - 最终升级 `go.mod` 到修复后的 Core 版本。
- 可选：
  - 抽取本地最小写出辅助以减少重复逻辑。
- 不做：
  - 不修改公开连接地址格式；
  - 不调整 await / auth 业务协议；
  - 不扩展扫描设备名能力。

## 可执行任务清单（Checklist）

### SDK-RFCOMM-1 - 修复 Session.Send 的整帧写出
- 目标：
  - 保证 SDK 在发送 `HeaderTcp` 帧时，即使底层 `Write` 短写，也能完整写完。
- 涉及模块 / 文件：
  - `session/session.go`
  - 可能新增共享写出辅助文件
- 验收条件：
  - `Session.Send` 不再依赖单次 `Write`；
  - 保持现有 `Connect` / `ConnectEndpoint` / `readLoop` 行为不变。
- 测试点：
  - fake short writer 覆盖；
  - session 关键路径测试通过。
- 回滚点：
  - 回退 session 发送实现。

### SDK-RFCOMM-2 - 升级 Core 依赖并对齐版本
- 目标：
  - 将 SDK 依赖的 `myflowhub-core` 升级到包含写出修复的新版本。
- 涉及模块 / 文件：
  - `go.mod`
  - `go.sum`
- 验收条件：
  - `go list -m github.com/yttydcs/myflowhub-core` 指向修复版本；
  - `GOWORK=off` 下可解析依赖。
- 测试点：
  - `go mod tidy`
  - `GOWORK=off go test ./... -count=1`
- 回滚点：
  - 回退依赖版本与 `go.sum`。

### SDK-RFCOMM-3 - Code Review（强制）
- 目标：
  - 逐项审查需求覆盖、架构一致性、性能影响、稳定性与测试覆盖。
- 涉及模块 / 文件：
  - 本 workflow 全部改动文件
- 验收条件：
  - 形成逐项通过/不通过结论；
  - 必要时返回实现阶段修正。
- 测试点：
  - Review 结论完整。
- 回滚点：
  - 修订实现或取消发版。

### SDK-RFCOMM-4 - 归档与发版准备
- 目标：
  - 生成独立归档文档，并准备 SDK patch 发布。
- 涉及模块 / 文件：
  - `docs/change/2026-03-15_sdk-rfcomm-write-contract-fix.md`
- 验收条件：
  - 文档可独立说明背景、修改、验证、影响与回滚；
  - 明确该修复面向 RFCOMM 与未来非 TCP 字节流。
- 测试点：
  - 归档文档完整可审计。
- 回滚点：
  - 回退归档文档。

## 依赖关系
- `SDK-RFCOMM-1` 完成后进入 `SDK-RFCOMM-2`
- `SDK-RFCOMM-2` 完成后进入 `SDK-RFCOMM-3`
- `SDK-RFCOMM-3` 通过后进入 `SDK-RFCOMM-4`

## 风险与注意事项
- SDK 不应复制 Core 的整套帧写出实现到过重程度，避免双份维护；
- 但在版本发布顺序上，SDK 仍需先能引用到新的 Core tag；
- 需要保持对 TCP 的完全兼容，不改变上层 API 与日志语义。
