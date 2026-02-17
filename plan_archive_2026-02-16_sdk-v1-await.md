# Plan - MyFlowHub-SDK v1（Awaiter：按 MsgID+SubProto+Action 等待响应）（PR9-SDK-Awaiter）

## Workflow 信息
- Repo：`MyFlowHub-SDK`
- 分支：`feat/sdk-v1-await`
- Worktree：`d:\project\MyFlowHub3\worktrees\pr9-sdk-v1-await\MyFlowHub-SDK`
- 参考总目标：`d:\project\MyFlowHub3\target.md`
- 接手者入口：`d:\project\MyFlowHub3\repos.md`
- 全局规范：`d:\project\MyFlowHub3\guide.md`（commit 信息中文）

## 当前状态
- SDK v0 已具备：
  - `session`：TCP Session（connect/close/send/readLoop），统一 HeaderTcp v2 编解码与默认字段补齐（`trace_id`/`hop_limit`）。
  - `transport`：`action+data` JSON envelope 的编码/解码（`EncodeMessage/DecodeMessage`）。
- 本 PR 不改 Win/Server；目标是把“等待响应/超时/取消”等一致语义下沉到 SDK（v1）。

---

## 1) 需求分析（已确认）

### 目标
1) 在 SDK 内新增通用 Awaiter（请求-响应等待语义）：支持 timeout/cancel、重复 key 防护、Close 清理防泄漏。
2) 本轮仅覆盖 Auth：`register/login/revoke`（wire 不改）。
3) 匹配粒度：`MsgID + SubProto + Action`（更安全，避免误投递）。
4) `SendAndAwait` 若请求 header 的 `MsgID==0`，SDK 自动生成非 0 的 MsgID 并写回 header。

### 范围（必须 / 可选 / 不做）
#### 必须（本 PR）
- 仅修改 `MyFlowHub-SDK` 仓库：新增 Awaiter + 与 `session.Session` 集成的发送/等待封装。
- 单测覆盖关键路径与边界（并发/超时/取消/重复 key/Close 清理）。

#### 可选（本 PR 如无额外风险）
- README 增加 v1 的概念使用方式示例（不引入强类型子协议 client）。

#### 不做（本 PR）
- 不改 `MyFlowHub-Proto` / `MyFlowHub-Core` / `MyFlowHub-Server` / `MyFlowHub-Win`。
- 不扩展到 VarStore/TopicBus/File/Flow 等其它子协议。
- 不引入强类型 Auth client（仅提供通用的 “send+await” 能力）。

### 验收标准
- `go test ./... -count=1 -p 1` 通过（Windows）。
- Awaiter 单测覆盖：
  - 正常投递
  - 超时
  - cancel
  - 重复 key 防护
  - Close 清理（等待者不泄漏）

### 风险
- 若某些响应缺少 `action` 或未携带可匹配的 `MsgID`，则无法按本策略 Await（本 PR 不处理该类 wire/Server 行为差异）。

---

## 2) 架构设计（已确认）

### 总体方案（含选型理由 / 备选对比）
#### 方案 A（采用）：SDK v1 新增 `await` 包（Broker + Client）
- 优点：不侵入 v0 的 `session`/`transport`；等待语义集中在 v1 包内，便于 Win/CLI 未来迁移。
- 关键约束：未匹配帧不吞，仍需回调给上层（避免影响通知/广播/日志帧）。

#### 方案 B（不采用）：把 Awaiter 做进 `session.Session`
- 风险：`session` 职责膨胀；onFrame 语义可能变得含糊（是否还能收到被 await 的响应）。

### 模块职责
- `await`：
  - `Broker`：按 key 注册等待/投递/取消/Close；并发安全；chan 缓冲 1，Deliver 不阻塞读循环。
  - `Client`：封装 `session.Session` 的 `SendAndAwait`；自动生成 MsgID；解析响应 action 并投递；未匹配帧继续转交回调。

### 数据 / 调用流（概念）
1) `await.NewClient(...)` 创建 client（内部持有 `session.Session` + `Broker`）。
2) `SendAndAwait(ctx, hdr, payload, expectAction)`：Register → Send → Wait（timeout/cancel/close）。
3) `session.readLoop` 收到帧触发 onFrame → `await.Client.handleFrame`：
   - 仅当存在等待者且 Major 为响应类时解析 payload.action
   - 匹配成功则 Deliver 并拦截（不再转交上层回调）
   - 否则转交上层回调（unmatched）

### 错误与安全
- 输入校验：header 必填、expectAction 必填；未连接发送返回错误。
- 重复 key：返回明确错误（避免覆盖导致错投递）。
- Close：唤醒并清理所有等待者，避免 goroutine/等待者泄漏。

### 性能关键点
- 只有当 (msg_id, sub_proto) 确实存在等待者时才执行 JSON 解码提取 `action`，避免对所有帧做 Decode。

---

## 3.1) 计划拆分（Checklist）

## 问题清单（阻塞：否）
- 无（范围/匹配策略/MsgID 自动生成已确认）。

### A1 - 新增 `await` 包（Broker）
- 目标：提供进程内 Await/Deliver 能力，key=(`MsgID`,`SubProto`,`Action`)。
- 涉及模块 / 文件：
  - `await/broker.go`
  - `await/broker_test.go`
- 验收条件：
  - Register/Deliver/Cancel/Close 语义清晰；并发安全；重复 key 防护；无等待者泄漏。
- 测试点：
  - 重复 key
  - cancel
  - Close 唤醒
  - 并发 Register/Deliver
- 回滚点：revert 本提交。

### A2 - 新增 `await` 包（Client：SendAndAwait + onFrame 集成）
- 目标：封装 `session.Session` 与 Broker，实现 `SendAndAwait`，并保证“未匹配帧不吞”。
- 涉及模块 / 文件：
  - `await/client.go`
  - `await/client_test.go`
- 验收条件：
  - `SendAndAwait` 对 `MsgID==0` 自动生成非 0 MsgID 并写回 header。
  - 匹配成功的响应帧被投递并拦截；未匹配帧按原样回调给上层。
  - Deliver 不阻塞 readLoop。
- 测试点：
  - 模拟服务端回包：匹配成功
  - action 不匹配导致超时（unmatched 回调仍收到帧）
  - ctx cancel
- 回滚点：revert 本提交。

### A3 - 文档补齐（README）
- 目标：说明 v1 Awaiter 的定位、边界与最小使用方式。
- 涉及文件：
  - `README.md`
- 验收条件：README 对 v0/v1 分层清晰；示例可读。
- 回滚点：revert 本提交。

### A4 - 回归与格式化
- 目标：保证代码质量与稳定。
- 验收条件：
  - `gofmt` 无差异
  - `go test ./... -count=1 -p 1` 通过
- 回滚点：revert 本 PR。

### A5 - Code Review + 归档
- 目标：可审计、可交接。
- 涉及文件：
  - `docs/change/2026-02-16_sdk-v1-await.md`
- 验收条件：包含需求覆盖、架构、性能、安全、测试结论与回滚方案。
- 回滚点：revert 归档提交（不影响实现）。

