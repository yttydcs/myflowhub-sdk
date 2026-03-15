# 变更背景 / 目标

Windows 蓝牙 RFCOMM 链路在“首次登录成功 → 断开 → 重连后登录失败”场景中，出现 `auth login: connection aborted`。
目标是让 SDK 会话断开后可稳定重连并继续 `SendAndAwait`。

# 具体变更内容

- `await/broker.go`
  - 新增 `Reopen()`，用于 reconnect 时清理 `closed` 状态并恢复可注册等待。
- `await/client.go`
  - `Connect()` 成功后调用 `broker.Reopen()`，确保断开后的 client 可复用。
- `session/session.go`
  - `Close()` 顺序调整为“先 cancel，再 close pipe”，避免断开竞态触发误报 `onError`。
- 新增测试
  - `await/broker_test.go`：`TestBroker_ReopenAfterClose`
  - `await/client_test.go`：`TestClient_ReconnectAfterClose_CanSendAndAwait`
  - `session/session_test.go`：`TestSessionClose_DoesNotReportOnError`

# 对应任务映射

- Task: RFCOMM 重连稳定性修复（SDK）
  - 目标：断开后重连登录恢复正常，不再残留 aborted 状态。
  - 验收：`go test ./...` 通过。

# 关键设计决策与权衡

- 保留 `Close()` 关闭等待者语义，但在 `Connect()` 显式 reopen broker，避免“连接可用但 await 永久不可用”的状态。
- 调整 `Session.Close()` 顺序为取消优先，避免 UI 上收到误导性的断开错误提示。

# 测试与验证

- `go test ./...`

# 潜在影响与回滚

- 影响：断开重连可复用同一 client，会话状态恢复策略更明确。
- 回滚：恢复 `Connect`/`Close` 旧逻辑并移除 reopen 相关改动。
