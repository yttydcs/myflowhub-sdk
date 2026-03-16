# 变更背景 / 目标

对齐 Core QUIC transport，SDK 需要支持 `quic://` endpoint，以便上层保持同一会话 API 即可连接 QUIC 链路。

# 具体变更内容

- 修改 `session/session.go`
  - `ConnectEndpoint` 新增 `quic://` 分支；
  - 通过 `quic_listener.DialEndpoint` 建立连接并接入现有 `Session` 读写链路。
- `go.mod` / `go.sum`
  - 升级 Core 到包含 QUIC 的版本（当前开发阶段使用 commit pseudo-version）；
  - 补齐 `quic-go` 相关校验依赖。

# 对应任务映射

- QUIC-SDK-1：SDK ConnectEndpoint 接入 `quic://`

# 关键设计决策与权衡

- 维持现有 `Session` API 与 `SendAndAwait` 行为，不引入新调用面；
- 仅新增 endpoint 分支，最小化对现有 TCP/RFCOMM 路径影响。

# 测试与验证

- `GOWORK=off go test ./...`

# 潜在影响与回滚

- 影响：新增 QUIC 依赖，其他传输行为保持不变；
- 回滚：移除 `quic://` 分支并回退依赖版本。
