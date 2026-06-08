# 2026-06-07 SDK typed subprotocol clients

## 变更背景 / 目标

SDK 已具备 `session`、`transport`、`await`，但调用方仍需要重复手写 HeaderTcp、JSON envelope、SubProto/action、目标节点与响应 decode 逻辑。这个重复点会让 Win、Android、CLI、脚本工具在业务协议上继续漂移。

本次目标是在 SDK 内新增首批低风险 typed client，覆盖 Management、Auth admin、VarStore、TopicBus，并保持 SDK 只依赖 Core、Proto、标准库。

## 具体变更内容

- 新增 `typed` 包：
  - `typed.New(*await.Client, typed.Options)`：复用已连接的 await client。
  - `Options{SourceID, TargetID}`：统一控制默认路由；`TargetID=0` 不作为上送语义。
  - 共享 helper：构造 `MajorCmd` HeaderTcp、封装 action/data、调用 `SendAndAwait` 或 `Send`、反序列化 typed response。
- 新增 Management typed client：
  - `NodeEcho`
  - `NodeInfo`
  - `ListNodes`
  - `ListSubtree`
  - `ConfigGet`
  - `ConfigSet`
  - `ConfigList`
- 新增 Auth admin typed client：
  - `GetPerms`
  - `ListRoles`
  - `ListPendingRegisters`
  - `ListRegisterPermits`
  - `ApproveRegister`
  - `RejectRegister`
  - `IssueRegisterPermit`
  - `RevokeRegisterPermit`
- 新增 VarStore typed client：
  - `Set`
  - `Get`
  - `List`
  - `Revoke`
  - `Subscribe`
  - `Unsubscribe`
- 新增 TopicBus typed client：
  - `Subscribe`
  - `SubscribeBatch`
  - `Unsubscribe`
  - `UnsubscribeBatch`
  - `ListSubs`
  - `Publish`
- 更新 README：
  - 标记 `typed` 为 v2 首批能力。
  - 说明 `TargetID=0` 的真实语义。
  - 说明 TopicBus publish no-ack、Auth 签名登录/注册不在本轮范围、File/Flow/Exec/Stream 未覆盖。
- 依赖对齐：
  - `github.com/yttydcs/myflowhub-core v0.4.9 -> v0.4.10`
  - `github.com/yttydcs/myflowhub-proto v0.1.5 -> v0.1.7`

## Requirements impact

none

本仓没有 SDK typed client 的稳定 `docs/requirements` 文档。本次实现的是 README 中既有演进方向：按小步增加子协议 client，不改变长期需求边界。

## Specs impact

none

未修改 wire 协议、SubProto 编号、action 字符串或服务端行为。实现依据以下现有 spec：

- `D:\project\MyFlowHub3\repo\MyFlowHub-Server\docs\specs\core.md`
- `D:\project\MyFlowHub3\repo\MyFlowHub-Server\docs\specs\protocol_map.md`
- `D:\project\MyFlowHub3\repo\MyFlowHub-Server\docs\specs\auth.md`
- `D:\project\MyFlowHub3\repo\MyFlowHub-Server\docs\specs\varstore.md`
- `D:\project\MyFlowHub3\repo\MyFlowHub-Server\docs\specs\topicbus.md`

## Lessons impact

none

本轮没有形成新的事故类或排障类复用经验。两个实现细节已在 plan 和本 archive 中记录：

- Auth `list_roles` 当前 Proto 有 `ListRolesReq` 与 `RolePermEntry`，但没有独立 response struct；SDK 以本地 `ListRolesResp` 对齐 `auth.md`。
- VarStore `unsubscribe` 当前没有稳定 `unsubscribe_resp` action；SDK 按 send-only 封装。

这些属于协议对齐记录，不单独创建 `docs/lessons`。

## Related requirements

- `README.md`

## Related specs

- `D:\project\MyFlowHub3\repo\MyFlowHub-Server\docs\specs\core.md`
- `D:\project\MyFlowHub3\repo\MyFlowHub-Server\docs\specs\protocol_map.md`
- `D:\project\MyFlowHub3\repo\MyFlowHub-Server\docs\specs\auth.md`
- `D:\project\MyFlowHub3\repo\MyFlowHub-Server\docs\specs\varstore.md`
- `D:\project\MyFlowHub3\repo\MyFlowHub-Server\docs\specs\topicbus.md`

## Related lessons

none

## 对应 plan.md 任务映射

- T1 - Shared typed client foundation: completed
- T2 - Management typed client: completed
- T3 - Auth admin typed client: completed
- T4 - VarStore typed client: completed
- T5 - TopicBus typed client: completed
- T6 - Align dependencies and documentation: completed
- T7 - Validate, review, and archive: completed through this archive

## 经验 / 教训摘要

- typed client 应作为 `await.Client` 的薄封装，不应重写 session、broker、MsgID 或 response matching。
- `TargetID=0` 不能包装为“上送父节点/authority”，否则会违反 Core 路由语义。
- TopicBus topic 不能 trim 或 normalize，因为 spec 要求原样匹配。
- VarStore `MajorCmd` response 不需要修改 typed client 或 await；现有 await 白名单已经覆盖。
- 对于 Proto 暂缺 response struct 的动作，优先以 SDK-local response struct 精确镜像 spec，而不是依赖 `map[string]any`。

## 可复用排查线索

- 症状：SDK typed client 调用超时。
  - 快速检查：确认 `SourceID` 和 `TargetID` 都是非 0 真实 NodeID；确认 response action 与 expected action 一致；确认响应携带非空 `data`。
- 症状：TopicBus publish 没有返回响应。
  - 快速检查：这是预期行为；publish 无 ack，应通过订阅方或 `onUnmatched` 观察事件。
- 症状：VarStore response 为 `MajorCmd` 时仍需 await。
  - 快速检查：确认使用现有 SDK await 版本，VarStore SubProto=3 的 `MajorCmd` response 已在 await 中白名单放行。
- 关键词：typed client, TargetID=0, list_roles response, varstore unsubscribe, topicbus publish no ack, MajorCmd response.

## 关键设计决策与权衡

- 选择 `typed` 包而不是应用侧 wrapper：减少 Win/Android/CLI 重复协议代码。
- 不实现 Auth signed register/login：避免密钥、签名、nonce 只做半套造成安全错觉。
- 不覆盖 File/Flow/Exec/Stream：这些协议含 data-plane、runtime、streaming 语义，适合后续独立 workflow。
- 协议 code 不自动转 Go error：Auth pending、VarStore not found、permission denied 等状态应由业务按 response code 判定；SDK 只把本地 validation、send、await、decode 失败作为 Go error。
- TopicBus `Publish` 不生成 MsgID：send-only 不经过 `SendAndAwait`，无 ack 场景不需要响应匹配。

## 测试与验证方式 / 结果

已通过：

```powershell
$env:GOWORK='off'; go test ./... -count=1 -p 1
```

结果：

- `github.com/yttydcs/myflowhub-sdk/await` ok
- `github.com/yttydcs/myflowhub-sdk/session` ok
- `github.com/yttydcs/myflowhub-sdk/transport` ok
- `github.com/yttydcs/myflowhub-sdk/typed` ok

已通过：

```powershell
git diff --check
```

Focused typed tests 覆盖：

- Management `node_echo` header/action/data 与 typed decode。
- Auth `list_roles` spec response shape。
- VarStore `set` 使用 `MajorCmd` response 且默认 visibility 为 `public`。
- TopicBus `publish` send-only 且 topic 原样保留。
- 空 response data 显式报错。
- nil await client、缺 target、空 config key、Auth permit 缺 role、VarStore invalid name/blank value/invalid subscriber、TopicBus blank publish name。

## 潜在影响

- 新增公开包 `github.com/yttydcs/myflowhub-sdk/typed`，后续调用方可能形成 API 兼容预期。
- SDK 依赖升级到 Core `v0.4.10`、Proto `v0.1.7`；发布态测试已覆盖。
- `go.sum` 移除了旧 Core/Proto checksum 并加入新版本 checksum。

## 回滚方案

- 删除 `typed/` 目录。
- 回退 README 中 v2 typed client 说明。
- 回退 `go.mod/go.sum` 的 Core/Proto 版本。
- 删除本 archive 与 worktree `plan.md`。

## 子Agent执行轨迹

none

已读 `$m-autoflow` sub-agent governance 并完成并行性评估。因为共享 helper/API 写集耦合高，未派发子Agent。

## Docs index note

SDK 当前仅有 `docs/change` 与 `docs/plan_archive` 历史目录，没有 category README。为避免把本轮 SDK typed client 实现扩大为 docs-tree repair，本次未新增或重建 docs index。
