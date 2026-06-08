# MyFlowHub-SDK（客户端 SDK）

`MyFlowHub-SDK` 是 MyFlowHub 体系里的 **客户端侧统一 SDK**（Go module：`github.com/yttydcs/myflowhub-sdk`）。

## 为什么需要 SDK

在“彻底重构”的目标态中：
- `MyFlowHub-Core` 负责底层框架能力（HeaderTcp v2、路由框架、连接/Process/Dispatcher 抽象等）
- `MyFlowHub-Proto` 负责纯协议字典（SubProto/Action/JSON struct，标准库依赖）
- `MyFlowHub-Server` 负责运行时装配与子协议 handler 的编排
- **客户端应用（例如 `MyFlowHub-Win`）不应重复实现协议机制**，而应通过统一 SDK 复用“连接/收发/默认字段/消息封装”等基础能力

SDK 的作用是把客户端侧容易漂移、容易复制粘贴的底层逻辑收敛起来，降低 Win/CLI/脚本工具的维护成本。

## 依赖方向（必须遵守）

- SDK 仅依赖：
  - `github.com/yttydcs/myflowhub-core`
  - `github.com/yttydcs/myflowhub-proto`
  - 标准库
- SDK **禁止**依赖 `myflowhub-server` 或 `myflowhub-win`（避免循环依赖与架构倒挂）。

## 当前能力

> 注：下文的 “v0/v1/v2” 是**能力阶段编号**（不是 semver tag）。

当前已包含：
- v0：`session`：TCP Session（connect/close/send/readloop），统一 HeaderTcp v2 编解码与默认字段补齐（`trace_id`/`hop_limit`）
- v0：`transport`：`action+data` JSON envelope 的编码/解码
- v1：`await`：通用 Awaiter（请求-响应等待语义：timeout/cancel/重复 req_id 防护等；已实现，见下文）
- v2：`typed`：首批强类型子协议 client（management / auth admin / varstore / topicbus；已实现，见下文）

未来演进（计划）：
- 按需继续增加 file / flow / exec / stream 等 typed client，但会坚持“小步多 PR”

## v1（已实现）：Awaiter（按 MsgID + SubProto + Action 等待响应）

v1 在 SDK 内新增 `await` 包，用于在客户端侧统一“请求-响应等待语义”：
- key：`MsgID + SubProto + Action`
- `SendAndAwait` 会在请求 header 的 `MsgID==0` 时自动生成非 0 MsgID 并写回 header
- 匹配成功的响应帧会被投递并拦截（不会重复转交给 onUnmatched）；未匹配帧不吞

## v2（首批已实现）：Typed 子协议 client

`typed` 包在 `await.Client` 之上提供低风险控制面子协议的强类型封装：
- `Management()`：`node_echo`、`node_info`、`list_nodes`、`list_subtree`、`config_get`、`config_set`、`config_list`
- `Auth()`：`register/login` ES256 helper，以及权限查询、角色列表、pending register、register permit、approve/reject 等管理动作
- `VarStore()`：`set/get/list/revoke/subscribe/unsubscribe`
- `TopicBus()`：`subscribe/unsubscribe/list_subs/publish`

边界：
- `TargetID=0` 在 Core 中表示“广播给子节点”，不是“上送父节点/authority”。typed client 的普通控制面方法默认要求 `SourceID` 与 `TargetID` 都是真实非 0 NodeID。
- `Auth().Register` / `Auth().Login` 是例外：默认使用未认证 direct bootstrap route（`SourceID=0, TargetID=0`），用于首次注册与登录；如需访问已知远端 authority，可在请求中显式传 route。
- `TopicBus.Publish` 无 ack，不会调用 `SendAndAwait`；异步 publish / notify 仍通过 `onUnmatched` 或 `SetOnFrame` 交给调用方处理。
- Auth key helper 使用 P256 + SHA256 + ECDSA ASN.1（`alg=ES256`），公钥/私钥均按 base64 DER 编码，文件 helper 对齐 `config/node_keys.json` 的 `privkey/pubkey` 字段。平台 keystore 或系统凭据保护仍由宿主应用负责。
- File / Flow / Exec / Stream typed client 暂未覆盖。

```go
c := await.NewClient(context.Background(),
  func(h core.IHeader, payload []byte) { /* unmatched frames */ },
  func(err error) { /* onError */ },
)
_ = c.Connect("127.0.0.1:9000")

tc := typed.New(c, typed.Options{SourceID: 10, TargetID: 1})

ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
defer cancel()

echo, err := tc.Management().NodeEcho(ctx, management.NodeEchoReq{Message: "ping"})
_ = echo
_ = err
```

> Auth register/login helper 示例

```go
c := await.NewClient(context.Background(), onUnmatched, onError)
_ = c.Connect("127.0.0.1:9000")

tc := typed.New(c, typed.Options{})
keys, err := typed.LoadOrCreateAuthKeyPair("config/node_keys.json")
_ = err

reg, err := tc.Auth().Register(ctx, typed.AuthRegisterRequest{
  DeviceID:    "win-001",
  DisplayName: "Win node",
  KeyPair:     keys,
})
_ = err

login, err := tc.Auth().Login(ctx, typed.AuthLoginRequest{
  DeviceID: "win-001",
  NodeID:   reg.NodeID,
  KeyPair:  keys,
})
_ = login
_ = err
```

## 快速示例（概念）

> 示例仅展示“如何组帧/封装 payload”，不代表完整业务流程（例如 login/register）。

```go
sess := session.New(context.Background(),
  func(h core.IHeader, payload []byte) { /* onFrame */ },
  func(err error) { /* onError */ },
)
_ = sess.Connect("127.0.0.1:9000")

payload, _ := transport.EncodeMessage("node_echo", map[string]any{"message": "ping"})
hdr := (&header.HeaderTcp{}).
  WithMajor(header.MajorCmd).
  WithSubProto(management.SubProtoManagement).
  WithSourceID(123).WithTargetID(123).
  WithMsgID(1).
  WithPayloadLength(uint32(len(payload)))

_ = sess.Send(hdr, payload)
```

> v1 示例：发送并等待响应（按 MsgID + SubProto + Action 匹配）

```go
c := await.NewClient(context.Background(),
  func(h core.IHeader, payload []byte) { /* unmatched frames */ },
  func(err error) { /* onError */ },
)
_ = c.Connect("127.0.0.1:9000")

payload, _ := transport.EncodeMessage("login", map[string]any{"device_id":"d","node_id":1})
hdr := (&header.HeaderTcp{}).WithMajor(header.MajorCmd).WithSubProto(2).WithSourceID(1).WithTargetID(0)

ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
defer cancel()
resp, err := c.SendAndAwait(ctx, hdr, payload, "login_resp")
_ = resp
_ = err
```

## 开发备注（联调与验收）

- 本地多仓联调：推荐使用工作区根目录的 `go.work`（不提交），以便跨仓联动开发。
- 单仓验收/发布验证：使用 `GOWORK=off go test ./... -count=1 -p 1`，确保无本地 replace 依赖。

