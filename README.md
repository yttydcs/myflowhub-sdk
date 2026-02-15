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

## 当前版本（v0）范围

本仓库当前聚焦 **最小可用 v0**：
- `session`：TCP Session（connect/close/send/readloop），统一 HeaderTcp v2 编解码与默认字段补齐（`trace_id`/`hop_limit`）
- `transport`：`action+data` JSON envelope 的编码/解码

后续演进（计划）：
- v1：引入通用 `Broker/Awaiter`（请求-响应等待语义：timeout/cancel/重复 req_id 防护等）
- v2：按需增加子协议 client（例如 management/auth/varstore 的强类型封装），但会坚持“小步多 PR”

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

## 开发备注（开发期）

本仓库在开发期可能使用 `go.mod` 的 `replace` 指向同级目录的 Core/Proto（便于多仓联调）。
后续会通过为 Core/Proto 打 tag 并移除 replace 来让仓库在“独立 clone”时也可直接构建。

