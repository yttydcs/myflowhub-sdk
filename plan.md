# Plan - MyFlowHub-SDK v0（Session/Transport 基础）（PR2-SDK-1）

## Workflow 信息
- Repo：`MyFlowHub-SDK`
- 分支：`feat/sdk-v0-session`
- Worktree：`d:\project\MyFlowHub3\worktrees\pr2-sdk-v0\MyFlowHub-SDK`
- 参考总目标：`d:\project\MyFlowHub3\target.md`
- 全局规范：`d:\project\MyFlowHub3\guide.md`（commit 信息中文）

## 当前状态
- `MyFlowHub-Win` 里已有最小的客户端实现：
  - `internal/session/session.go`：Connect/Send/readLoop + trace_id 生成
  - `internal/services/transport/codec.go`：`action+data` JSON envelope 编码
- 目标态约束（已确认）：Win 作为更上层应用，后续应通过统一 SDK/Client 调用能力，而非重复实现协议机制。

---

## 1) 需求分析

### 目标
1) 创建独立仓库 `MyFlowHub-SDK`（Go module：`github.com/yttydcs/myflowhub-sdk`），提供 **客户端侧最小可复用基础能力**。
2) SDK v0 聚焦“底层传输一致性”：
   - Session：connect/close/send/readloop（HeaderTcp v2）
   - transport：`action+data` JSON envelope 编码/解码
3) 保持与现有 Win 行为一致（尤其默认 `trace_id`、`hop_limit` 的补齐规则），以便下一步小改动迁移 Win。

### 范围（必须 / 可选 / 不做）
#### 必须（本 PR）
- 建立仓库骨架：`go.mod`、`README.md`、`.gitignore`、`plan.md`
- `transport` 包：
  - `Message`（`action + data`）
  - `EncodeMessage(action, data)`（输入校验）
  - `DecodeMessage(payload)`（解析并返回 `action` 与 `json.RawMessage`，避免二次反序列化）
- `session` 包：
  - `Session`：Connect/Close/Send/readLoop
  - Send 时补齐：`hop_limit`（默认 16）与 `trace_id`（随机/递增 uint32，0 视为未设置）
  - 可注入回调：`onFrame(hdr, payload)`、`onError(err)`
- 单元测试覆盖关键路径与边界（net.Pipe / bufio.Reader）
- 回归：`go test ./... -count=1 -p 1` 通过

#### 可选（本 PR 如无额外风险）
- 提供 `session.Options`（dial timeout / logger / trace id generator）以便未来扩展

#### 不做（本 PR）
- 不做 request/response 等待语义（Broker/Awaiter）——留到 SDK v1（单独 PR）
- 不提供各子协议的“强类型 client”（auth/management/varstore…）——留到后续按需加
- 不改 Core/Proto/Win/Server 任何代码（本 PR 只落 SDK 仓库）

### 使用场景
- Win/CLI/脚本工具作为客户端连接 HubServer：
  - 统一 HeaderTcp v2 组帧/解帧
  - 统一 trace_id/hop_limit 默认值
  - 统一 envelope 编码（`action+data`）

### 功能需求
- 能连接 TCP 地址并启动读循环，把帧解码交给回调处理
- 能发送指定 header+payload 的帧，并在缺省字段上填充默认值
- 能把 `{action,data}` 编码成 JSON（以及从 JSON 解出 action+raw data）

### 非功能需求
- 性能：避免不必要的重复 marshal/unmarshal；读循环使用 `bufio.Reader`
- 可维护性：API 与 Win 现有最小实现保持近似，迁移成本低
- 安全默认：输入校验（action 必填；空 payload 解码容错）
- 可扩展性：预留 Options 与错误类型扩展点，不引入破坏性依赖

### 输入输出（API 级别）
- 输入：
  - `Connect(addr)`：`addr`（host:port）
  - `Send(hdr, payload)`：HeaderTcp v2 及 payload bytes
  - `EncodeMessage(action, data)`：action string + data(any)
- 输出：
  - Connect/Send 返回 error
  - readLoop 通过 `onFrame(hdr, payload)` 输出收到的帧
  - `DecodeMessage` 返回解析后的 message（action + raw data）

### 边界异常
- 重复 Connect
- Close 后再次 Connect
- 网络读写错误（触发 onError，并退出读循环）
- DecodeMessage 输入不是 JSON / 缺少 action

### 验收标准
- `go test ./...` 通过
- `transport` 与 `session` 的关键行为有单测：
  - Send 自动补齐 trace_id/hop_limit
  - Encode/DecodeMessage 输入校验与 roundtrip
- README 清晰说明定位、依赖方向、v0/v1 演进计划

### 风险
- SDK 作为公共依赖，API 一旦发布后变更成本高；因此 v0 必须“最小而正确”

---

## 2) 架构设计（分析）

### 总体方案（含选型理由 / 备选对比）
#### 方案 A（采用）：最小 SDK v0（Session + transport）
- 优点：迁移 Win 成本最低；可快速统一协议收发与默认字段；利于小步多 PR
- 缺点：调用方仍需自己做“等待响应/超时/取消”等语义（但这是可在 v1 统一下沉的）

#### 方案 B（不采用，本 PR 不做）：直接引入 Broker/Awaiter
- 优点：调用方更简单
- 缺点：设计面更大，容易一次做过头；需要确定 req_id 生成/匹配策略与跨子协议的一致规则

### 模块职责
- `transport`：`action+data` envelope 编解码（不关心网络）
- `session`：TCP 连接 + HeaderTcp v2 编解码 + 读循环 + 默认字段补齐

### 数据 / 调用流
1) 调用方 `session.New(...)` 创建 Session，传入 onFrame/onError
2) `Connect(addr)` 建立 TCP 并启动 readLoop
3) readLoop：`HeaderTcpCodec.Decode` → `onFrame(hdr, payload)`
4) 调用方构造 header/payload（payload 可由 `transport.EncodeMessage` 产生）→ `Send(hdr, payload)`

### 接口草案
- `transport`：
  - `type Message struct { Action string; Data json.RawMessage }`
  - `func EncodeMessage(action string, data any) ([]byte, error)`
  - `func DecodeMessage(payload []byte) (Message, error)`
- `session`：
  - `type Session struct { ... }`
  - `func New(ctx context.Context, onFrame func(core.IHeader, []byte), onError func(error)) *Session`
  - `func (s *Session) Connect(addr string) error`
  - `func (s *Session) Close()`
  - `func (s *Session) Send(hdr core.IHeader, payload []byte) error`

### 错误与安全
- `transport`：action 为空直接返回 error；DecodeMessage 缺 action 返回 error
- `session`：未连接发送返回 error；读循环遇到解码/网络错误回调 onError 并退出

### 性能与测试策略
- 关键点：
  - DecodeMessage 使用 `json.RawMessage` 承载 data，避免多一次 unmarshal
  - 读循环使用 `bufio.Reader`（减少系统调用）
- 测试：
  - `transport`：Encode/Decode 的输入校验与 roundtrip
  - `session`：net.Pipe 下验证 Send 的 header 默认字段补齐；验证 readLoop 分发

### 可扩展性设计点
- 预留 `session.Options`：dial timeout、logger、trace id generator（便于测试与未来可观测）
- SDK v1 可在不破坏 v0 的前提下增加：
  - `Broker/Awaiter`
  - 子协议 client（按需逐步增加）

---

## 3.1) 计划拆分（Checklist）

## 问题清单（阻塞：否）
- 无（范围、依赖与验收已明确）。

### S1 - 初始化仓库骨架
- 目标：可被 `go test ./...` 扫描，README 明确定位与依赖方向
- 涉及文件：
  - `go.mod`
  - `README.md`
  - `.gitignore`
- 验收条件：`go test ./...` 可运行（即使暂时无包也应通过）
- 测试点：无
- 回滚点：revert 本提交

### S2 - 新增 transport 包（envelope 编解码）
- 目标：统一 `{action,data}` 的编码与解析
- 涉及文件：
  - `transport/message.go`
- 验收条件：输入校验完善；单测覆盖
- 测试点：action 为空、payload 非 JSON、缺 action、正常 roundtrip
- 回滚点：revert 本提交

### S3 - 新增 session 包（connect/send/readloop）
- 目标：抽取 Win 内部 session 逻辑为可复用 SDK
- 涉及文件：
  - `session/session.go`
- 验收条件：Send 自动补齐 hop_limit/trace_id；读循环可分发帧
- 测试点：net.Pipe + codec Decode 验证 header 字段
- 回滚点：revert 本提交

### S4 - 全量回归与格式化
- 目标：确保代码质量与稳定
- 验收条件：
  - `gofmt` 无差异
  - `go test ./... -count=1 -p 1` 通过
- 回滚点：revert 本 PR

### S5 - Code Review + 归档
- 目标：可审计、可交接
- 涉及文件：
  - `docs/change/2026-02-15_sdk-v0-session.md`
- 验收条件：包含需求覆盖、架构、性能、安全、测试结论与回滚方案
- 回滚点：revert 归档提交（不影响实现）

