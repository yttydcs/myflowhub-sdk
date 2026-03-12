# SDK：支持 `bt+rfcomm://`（Bluetooth Classic RFCOMM）Dial（重大变更）

## 背景 / 目标
- 背景：SDK 侧原先仅支持 TCP（`net.Dial("tcp")` + `net.Conn`），无法接入 RFCOMM 等非 TCP 承载。
- 目标：
  - 对齐 Server 的 endpoint 规范，使客户端可通过 `bt+rfcomm://...` 建立连接；
  - 复用 Core 的 RFCOMM dial，实现与 TCP 类似的 connect/收发帧能力；
  - 保持帧语义不变（仍使用 HeaderTcpCodec）。

## 变更内容
- `session/session.go`
  - 增加 endpoint scheme 分发：`tcp` / `bt+rfcomm`
  - RFCOMM 侧复用 `myflowhub-core/listener/rfcomm_listener.DialEndpoint`
- `go.mod` / `go.sum`
  - 依赖升级以对齐 Core 的 RFCOMM 能力（开发期可通过 workflow-local go.work 联动）
- 单测
  - 覆盖 endpoint 解析与错误路径（无真实蓝牙环境下至少能返回可读错误）

## 关键设计决策与权衡
- **连接底座**：以 `Pipe(io.ReadWriteCloser)` 语义对齐 Core，避免强依赖 `net.Conn`（为未来更多承载留空间）。
- **可维护性**：endpoint scheme 分发集中在 Session 层，Core 负责各承载 dial/listen 细节。

## 测试与验证
- `go test ./... -count=1`

## 回滚方案
- revert 本次提交；客户端继续使用 TCP connect。

