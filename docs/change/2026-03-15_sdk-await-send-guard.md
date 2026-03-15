# 变更背景 / 目标

在 `SendAndAwait` 场景中，若底层 `sess.Send` 卡在写入阶段，上层超时无法生效，导致 UI 与业务调用链长时间无返回。

# 具体变更内容

- 修改 `await/client.go`
  - `SendAndAwait` 改为通过 `sendWithContext` 发送。
  - 新增 `sendWithContext`：
    - 用 goroutine 执行底层发送；
    - 若 `ctx.Done()` 先触发且发送仍未返回，主动 `sess.Close()` 中断阻塞写；
    - 返回带上下文取消/超时语义的错误。

# 对应任务映射

- Task: SDK await 发送阶段超时兜底
  - 目标：保证上层超时可达，不再无限挂起。
  - 文件：`await/client.go`
  - 验收：`go test ./await` 通过；`go test ./...` 通过。

# 关键设计决策与权衡

- 发送卡死时主动关闭 session，优先保证系统可恢复与可观察性。
- 代价是超时时连接会被关闭，需要上层重连；但比无限挂起更可控。

# 测试与验证

- `go test ./await`
- `go test ./...`

# 潜在影响与回滚

- 影响：在发送阶段触发超时/取消时，连接会进入关闭路径。
- 回滚：恢复 `SendAndAwait` 直接调用 `sess.Send` 的旧逻辑。
