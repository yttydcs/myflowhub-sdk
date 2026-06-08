# 2026-06-08 SDK Auth signing helpers

## 变更背景 / 目标

首批 SDK typed client 已覆盖 Management、Auth admin、VarStore、TopicBus，但 Auth `register/login` 的 P256 key、DER/base64 编码、nonce、ES256 签名、bootstrap route 和 JSON envelope 仍需要 Win、Android、CLI 等宿主各自实现。

本轮目标是在 SDK 内完整收敛 Auth direct register/login 的客户端 helper，让应用可以复用同一套协议实现，同时不改变 Server/SubProto/Core/Proto 的 wire contract。

## 具体变更内容

- 新增 Auth key/signing primitives：
  - `typed.GenerateAuthKeyPair`
  - `typed.AuthKeyPairFromPrivateKey`
  - `typed.ParseAuthPrivateKeyBase64`
  - `typed.ParseAuthPublicKeyBase64`
  - `typed.EncodeAuthPrivateKeyBase64`
  - `typed.EncodeAuthPublicKeyBase64`
  - `typed.ReadAuthKeyPair`
  - `typed.WriteAuthKeyPair`
  - `typed.LoadOrCreateAuthKeyPair`
  - `typed.GenerateAuthNonce`
  - `typed.LoginSignBytes`
  - `typed.SignLogin`
- 新增 Auth register/login request structs：
  - `typed.AuthRegisterRequest`
  - `typed.AuthLoginRequest`
- 新增 high-level typed Auth methods：
  - `tc.Auth().Register(ctx, typed.AuthRegisterRequest{...})`
  - `tc.Auth().Login(ctx, typed.AuthLoginRequest{...})`
- 新增 method-scoped route helper：
  - 普通 typed request-response methods 继续要求 `SourceID != 0` 且 `TargetID != 0`。
  - `Auth().Register` / `Auth().Login` 默认走 direct bootstrap `SourceID=0, TargetID=0`。
  - 高级调用方可通过请求里的 `Route *typed.Options` 显式指定 authority route。
- 更新 README：
  - Auth typed client 现已覆盖 `register/login` ES256 helper。
  - 文档说明 scoped bootstrap route 例外、key 文件格式、平台 keystore 边界。
- 新增测试：
  - key generation / parse round trip。
  - `node_keys.json` read/write/load-or-create 行为。
  - malformed existing key file does not regenerate。
  - exact `LoginSignBytes` string。
  - ES256 signature standard-library verification。
  - register wire header/action/data with `SourceID=0, TargetID=0`。
  - login wire header/action/data and verifiable signature。
  - missing field, route, key, timestamp, nonce, alg validation failures。
  - Auth admin route validation remains strict.

## Requirements impact

none

本仓仍没有稳定 `docs/requirements` 树。本轮实现的是 SDK README 中 typed client 演进方向的下一步，未新增跨模块业务需求文档。

## Specs impact

none

未修改 Auth wire protocol、SubProto 编号、action 字符串、Proto struct 或 Server/SubProto 行为。实现严格按以下现有技术契约落地：

- `D:\project\MyFlowHub3\repo\MyFlowHub-Server\docs\specs\auth.md`
- `D:\project\MyFlowHub3\repo\MyFlowHub-Proto\protocol\auth\types.go`
- `D:\project\MyFlowHub3\repo\MyFlowHub-SubProto\auth\crypto.go`
- `D:\project\MyFlowHub3\repo\MyFlowHub-SubProto\auth\actions_register.go`
- `D:\project\MyFlowHub3\repo\MyFlowHub-SubProto\auth\actions_login.go`
- `D:\project\MyFlowHub3\repo\MyFlowHub-Android\hubmobile\keys.go`
- `D:\project\MyFlowHub3\repo\MyFlowHub-Win\internal\services\auth\keys.go`

## Lessons impact

none

本轮没有新增事故类排查结论。两个可复用注意点已记录在本 archive 和 README：

- `TargetID=0` 仍不是通用上送语义；只有 Auth direct register/login helper 默认使用 `0/0` bootstrap route。
- key 文件存在但损坏时 SDK 不静默覆盖，避免误换身份密钥。

## Related requirements

- `README.md`

## Related specs

- `D:\project\MyFlowHub3\repo\MyFlowHub-Server\docs\specs\auth.md`
- `D:\project\MyFlowHub3\repo\MyFlowHub-Proto\protocol\auth\types.go`
- `D:\project\MyFlowHub3\repo\MyFlowHub-SubProto\auth\crypto.go`
- `D:\project\MyFlowHub3\repo\MyFlowHub-SubProto\auth\actions_register.go`
- `D:\project\MyFlowHub3\repo\MyFlowHub-SubProto\auth\actions_login.go`
- `D:\project\MyFlowHub3\repo\MyFlowHub-Android\hubmobile\keys.go`
- `D:\project\MyFlowHub3\repo\MyFlowHub-Win\internal\services\auth\keys.go`

## Related lessons

none

## 对应 plan.md 任务映射

- T8 - Add Auth key and ES256 signing primitives: completed
- T9 - Add Auth register/login typed helpers with scoped bootstrap routing: completed
- T10 - Add Auth crypto/wire tests and update README: completed
- T11 - Re-run validation, review, and archive: completed through this archive

## 经验 / 教训摘要

- SDK 可以收敛重复的 client-side crypto contract，但不能依赖 Server/SubProto 包；本轮只使用 Core、Proto、标准库。
- Auth register 当前不要求签名，SDK 不应发明额外字段或本地签名语义。
- Login signing bytes 必须与 SubProto/Android/Win 完全一致：`login\n<device_id>\n<node_id>\n<ts>\n<nonce>`。
- `display_name` 是可选提示字段，不进入 login signature digest。
- Direct bootstrap 的 `SourceID=0, TargetID=0` 是 Auth register/login 的 scoped exception，不能扩散到 Auth admin 或其他 typed clients。
- `LoadOrCreateAuthKeyPair` 对损坏的既有 key file 显式失败，避免静默替换导致节点身份变化。

## 可复用排查线索

- 症状：`Auth().Login` 被 authority 拒绝为 invalid signature。
  - 快速检查：确认 `device_id/node_id/ts/nonce` 与签名时完全一致；确认 `display_name` 没有参与签名；确认 key pair 是 P256 且 public key 已注册。
- 症状：普通 typed Auth admin 方法报 `source_id is required` 或 `target_id is required`。
  - 快速检查：这是预期 guard；只有 `Register/Login` 默认允许 `0/0` bootstrap。
- 症状：`LoadOrCreateAuthKeyPair` 在 key 文件存在时失败。
  - 快速检查：读取 `config/node_keys.json` 是否是 `{"privkey":"...","pubkey":"..."}`，且二者是否同属一个 P256 key pair；SDK 不会覆盖损坏文件。
- 关键词：Auth register, Auth login, ES256, P256, node_keys.json, loginSignBytes, SourceID=0, TargetID=0, bootstrap route.

## 关键设计决策与权衡

- 使用 request structs 而不是位置参数，给 `display_name`、`join_permit`、`requested_role`、fixed `ts/nonce`、explicit route 留出清晰扩展点。
- 保留 low-level crypto helpers，方便宿主自行管理密钥或做 deterministic tests。
- 不自动把 protocol `data.code` 转 Go error；`pending/rejected/authority unavailable` 等仍由调用方按 `auth.RespData` 判断。
- 不引入平台 keystore；SDK 只提供跨平台 key file helper，安全存储策略归宿主应用负责。
- 不实现 `assist_*`、`up_login`、`revoke/offline`；这些涉及多跳 authority 与 sender signature，应单独规划。

## 测试与验证方式 / 结果

已通过：

```powershell
$env:GOWORK='off'; go test ./typed -count=1
```

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

## 潜在影响

- `typed` 包新增公开 Auth helper API，后续调用方可能形成兼容预期。
- Auth direct register/login 现在可以直接复用 SDK；Win/Android 后续可逐步迁移并删除本地重复 crypto code。
- key file helper 写入 `0o600` 权限；Windows 上权限语义由 Go/OS 实际支持决定。

## 回滚方案

- 删除 `typed/auth_keys.go`。
- 删除 `typed/auth_keys_test.go` 与 `typed/auth_wire_test.go`。
- 回退 `typed/auth.go` 中 register/login request structs 和 methods。
- 回退 `typed/client.go` 中 `sendAndDecodeWithOptions` / `headerWithOptions` 相关改动。
- 回退 README 的 Auth register/login helper 说明。
- 删除本 archive。

## 子Agent执行轨迹

none

已读 `$m-autoflow` sub-agent governance 并完成并行性评估。因为 Auth public API、scoped route helper、crypto helpers、tests、README 属于同一兼容性表面，未派发子Agent。

## Docs index note

SDK 当前仅有 `docs/change` 与 `docs/plan_archive` 历史目录，没有 category README。为避免把本轮 Auth helper 实现扩大为 docs-tree repair，本次未新增或重建 docs index。
