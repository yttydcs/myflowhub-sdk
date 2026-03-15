# SDK：修复 RFCOMM 客户端帧发送短写问题

## 变更背景 / 目标
- 背景：
  - Win 客户端通过 SDK 走 RFCOMM 连接时，虽然 UI 能显示已连接，但 `register` 帧发送后服务端无法稳定收到完整帧；
  - 根因是 SDK `Session.Send` 直接做一次 `pipe.Write(frame)`，没有保证完整帧写完；
  - Core 已在 `v0.4.3` 修复服务端 / 通用传输层写出契约，SDK 需要同步对齐。
- 目标：
  - 修复 SDK 客户端发送路径的短写风险；
  - 升级到 `myflowhub-core v0.4.3`；
  - 为 Win release 提供可用依赖版本。

## 具体变更内容
- 修改：
  - `session/session.go`
    - `Session.Send` 改为调用 `core.WriteAll(pipe, frame)`，保证整帧写出
  - `go.mod`
    - `github.com/yttydcs/myflowhub-core` 升级到 `v0.4.3`
  - `go.sum`
    - 更新依赖校验
  - `session/session_test.go`
    - 增加短写 pipe 回归测试

## 对应 plan.md 任务映射
- `SDK-RFCOMM-1`：完成 Session 发送路径修复
- `SDK-RFCOMM-2`：完成 Core 依赖升级
- `SDK-RFCOMM-3`：已执行代码审查
- `SDK-RFCOMM-4`：本文档

## 关键设计决策与权衡
- 复用 Core 的统一写出契约，而不是在 SDK 再维护一套独立短写补丁；
- 这样 client/server 两端语义保持一致，后续 transport 扩展点也更容易维护；
- 性能影响极低：仅在底层出现短写时才额外补写。

## 测试与验证方式 / 结果
- 已执行：
  - `GOWORK=off go test ./... -count=1`
- 结果：
  - 通过
- 重点覆盖：
  - SDK session 在短写 pipe 上仍能完整发送一帧

## 潜在影响与回滚方案
- 潜在影响：
  - 对 TCP 无行为变化，只提升健壮性；
  - RFCOMM 与未来其他非 TCP 字节流承载的发送稳定性提升。
- 回滚方案：
  - 回退以下文件：
    - `session/session.go`
    - `session/session_test.go`
    - `go.mod`
    - `go.sum`
