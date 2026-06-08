# Plan - SDK typed subprotocol clients

## Workflow Information
- Repo: `D:\project\MyFlowHub3\repo\MyFlowHub-SDK`
- Active worktree: `D:\project\MyFlowHub3\worktrees\MyFlowHub-SDK-feat-sdk-typed-clients`
- Branch: `feat/sdk-typed-clients`
- Base: `main` at `578ed3a docs: 补充源码理解注释`
- Current Stage: `3.1 - Planning (scope expansion: Auth register/login ES256 helpers)`
- Workflow skill: `$m-autoflow`
- Docs routing skill: `$m-docs`

## Stage Records

### Initialization
- `guide.md`: read. Key constraints recorded:
  - all worktrees must live under `D:\project\MyFlowHub3\worktrees`
  - subprotocol docs are under `repo\MyFlowHub-Server\docs`
  - commit messages should use Chinese text, with English type prefix allowed
  - after implementation / validation / commit / closeout, send MyFlowHub MCP notification if available
- Participating repo: `MyFlowHub-SDK`
- Participating modules: SDK `await`, `transport`, new typed client layer, README, tests, `go.mod/go.sum`
- Main repo status before worktree: clean, `main...origin/main`
- Worktree status before planning: clean, `feat/sdk-typed-clients`
- Implementation is forbidden in `repo\MyFlowHub-SDK`; all edits must stay inside the active worktree.

### Stage 1 - Requirements Analysis

#### Goal
Add the first SDK typed client layer for low-risk MyFlowHub control-plane subprotocols so Win, Android, CLI, and scripts do not need to repeatedly hand-code HeaderTcp, JSON envelope, await matching, and typed response decoding.

#### Scope
Must:
- Add typed clients in `MyFlowHub-SDK`, reusing existing `await.Client`, `transport`, Core headers, and Proto types.
- Cover:
  - Management: `node_echo`, `node_info`, `list_nodes`, `list_subtree`, `config_get`, `config_list`, and explicit `config_set`.
  - Auth admin/control: `get_perms`, `list_roles`, `list_pending_registers`, `list_register_permits`, `approve_register`, `reject_register`, `issue_register_permit`, `revoke_register_permit`.
  - VarStore: `set`, `get`, `list`, `revoke`, `subscribe`, `unsubscribe`.
  - TopicBus: `subscribe`, `subscribe_batch`, `unsubscribe`, `unsubscribe_batch`, `list_subs`, `publish`.
- Preserve SDK dependency direction: Core + Proto + standard library only.
- Update README with usage and scope.
- Add tests for header/action/data construction, typed response decoding, validation errors, TopicBus no-ack publish, and VarStore `MajorCmd` response delivery.

May:
- Align SDK dependency versions with the current Server/Proto matrix: `myflowhub-core v0.4.10`, `myflowhub-proto v0.1.7`.
- Add a shared options/client entrypoint to reduce repeated source/target parameters.

Will not:
- Implement full Auth register/login signing, key storage, nonce generation, or ES256 signature helpers.
- Implement File, Flow, Exec, or Stream typed clients in this workflow.
- Modify Server, SubProto, Core, or Proto behavior.
- Migrate Win/Android call sites.
- Treat `TargetID=0` as "send to parent/authority".

#### Use Cases
- A host app with known `source node id` and target hub/authority id calls SDK methods for management, auth administration, varstore, and topicbus without custom JSON maps.
- A CLI or test tool connects to `127.0.0.1:9000` and receives Proto response structs.
- Applications still receive asynchronous frames through `onUnmatched` / `SetOnFrame`.

#### Functional Requirements
- Request-response methods must construct `MajorCmd + SubProto + SourceID + TargetID`, encode `{"action","data"}`, and call `SendAndAwait`.
- `TopicBus.Publish` must use `Send` only and must not wait for an ack.
- Responses must decode `await.Response.Message.Data` into the corresponding Proto response type.
- Validate obvious protocol and routing errors before sending.
- Keep TopicBus topic strings unchanged; do not trim or normalize them.
- For VarStore, validate `name`, `set.value`, `visibility`, and subscribe `subscriber` semantics.
- For Auth admin, require non-empty request IDs, device IDs, roles, and permit tokens where the spec requires them.

#### Non-functional Requirements
- No dependency on Server, SubProto, Win, or Android.
- No breaking changes to existing `session`, `transport`, or `await` public APIs.
- Explicit errors for validation, send, timeout/cancel, and response decode failures.
- `GOWORK=off go test ./... -count=1 -p 1` must pass to prove release-mode dependencies.

#### Inputs / Outputs
- Inputs: connected `*await.Client`, context, source node id, target node id, and Proto request structs or typed params.
- Outputs: Proto response structs and Go errors for local/transport/decode failures.
- No-ack output: `TopicBus.Publish` returns only send/validation errors.

#### Edge Cases
- `SourceID=0 && SubProto!=Auth` is dropped by Core; SDK should fail early.
- `TargetID=0` is child broadcast, not upstream; typed control-plane request methods should fail early.
- VarStore responses may use `MajorCmd`; existing await whitelist must be preserved.
- Auth approve only reserves identity; the requester must retry register to complete admission.
- TopicBus publish has no ack and no historical replay.

#### Acceptance Criteria
- Tests assert outbound headers and payload action/data for each subclient group.
- Tests cover typed response decoding and validation failures.
- Tests cover TopicBus publish without waiting for response.
- Tests cover VarStore `MajorCmd` response decode through typed client.
- README documents usage, target id semantics, and deferred Auth signing/File/Flow/Exec/Stream scope.
- `GOWORK=off go test ./... -count=1 -p 1` passes.
- `git diff --check` passes.

#### Risks
- API shape will become a compatibility surface; keep it small and explicit.
- Hidden target inference could break remote authority / parent routing; avoid it.
- Partial Auth signing helpers would create a security false sense; defer them.
- TopicBus topic normalization would violate the spec; preserve raw topic strings.
- Leaving dependencies behind Server's Proto/Core matrix could keep cross-app drift alive.

#### Issue List
- None currently blocking Stage 3.1.

### Stage 2 - Architecture Design

#### Overall Solution
Add a thin typed client layer above `await.Client`. The new layer owns request validation, HeaderTcp construction, JSON envelope encoding, response action selection, and Proto response decoding. It does not change session, transport, or await behavior.

Selection rationale:
- `await.Client` already provides MsgID generation, context cancellation, response matching by `MsgID + SubProto + Action`, reconnect broker reopen, and VarStore `MajorCmd` response compatibility.
- Proto already defines stable action constants and request/response structs.
- Keeping this in SDK avoids application copy/paste without introducing Server/SubProto dependencies.

#### Alternatives Considered
- Continue app-local wrappers: rejected because it repeats protocol details across Win/Android/CLI.
- Generate clients from Server/SubProto: rejected because it risks dependency inversion and larger build coupling.
- Cover all subprotocols now: rejected because File/Flow/Exec/Stream include data-plane, runtime, or streaming semantics outside the safe MVP.

#### Module Responsibilities
- `await`: unchanged request-response matcher.
- `transport`: unchanged envelope codec.
- New typed layer:
  - shared request helper
  - Management subclient
  - Auth admin subclient
  - VarStore subclient
  - TopicBus subclient
- README and tests document and lock the public behavior.

#### Data / Call Flow
1. Caller creates and connects `await.Client`.
2. Caller creates typed client with `SourceID` and `TargetID`.
3. Subclient validates request and routing fields.
4. Subclient builds HeaderTcp and envelope.
5. Request-response methods call `SendAndAwait`; publish calls `Send`.
6. Subclient decodes `Message.Data` into Proto response structs.
7. Asynchronous frames remain visible to existing callbacks.

#### Interface Drafts
```go
c := await.NewClient(ctx, onUnmatched, onError)
err := c.Connect("127.0.0.1:9000")

tc := typed.New(c, typed.Options{SourceID: 10, TargetID: 1})

echo, err := tc.Management().NodeEcho(ctx, management.NodeEchoReq{Message: "ping"})
perms, err := tc.Auth().GetPerms(ctx, auth.PermsQueryData{NodeID: 10})
vr, err := tc.VarStore().Get(ctx, varstore.GetReq{Name: "temp", Owner: 10})
err = tc.TopicBus().Publish(ctx, topicbus.PublishReq{Topic: "dev/codex", Name: "event", TS: now, Payload: raw})
```

The exact package name remains part of implementation, but it must avoid import cycles and should keep call sites clear.

#### Error Handling and Safety
- Local validation errors are returned before sending.
- Send, await timeout/cancel, closed client, and decode errors are returned as Go errors.
- Protocol `data.code` values are returned in response structs, not automatically converted to Go errors in this first version. This preserves Auth `202 pending`, VarStore `4 not found`, and permission codes for callers.
- Non-Auth control-plane methods require positive SourceID and TargetID.
- No hidden target resolver is introduced.

#### Performance and Testing Strategy
- One marshal and one unmarshal per request-response method.
- No new background goroutines beyond existing await/session behavior.
- Use fake TCP tests to inspect actual wire headers and payloads.
- Use release-mode validation with `GOWORK=off`.

#### Extensibility Design Points
- Shared helper should accept subproto, action, response action, and typed decode target.
- Future Flow/Exec/File/Stream clients can reuse the helper but must be planned separately.
- Options may later grow timeout/header hook/target resolver, but this workflow should keep it minimal.

#### Issue List
- None currently blocking Stage 3.1.

### Stage 3.1 - Planning

#### Project Goal and Current State
- Current SDK has `session`, `transport`, and `await`, plus README stating future v2 subprotocol clients should be added in small steps.
- Current gap: apps can connect and await responses, but still need to rewrite business protocol wrappers.
- This plan implements the first low-risk typed client set while preserving current SDK boundaries.

#### Docs Governance Routing Decision
- Used `$m-docs` for plan routing and impact checks.
- Canonical stable technical contracts remain in:
  - `D:\project\MyFlowHub3\repo\MyFlowHub-Server\docs\specs\core.md`
  - `D:\project\MyFlowHub3\repo\MyFlowHub-Server\docs\specs\protocol_map.md`
  - `D:\project\MyFlowHub3\repo\MyFlowHub-Server\docs\specs\auth.md`
  - `D:\project\MyFlowHub3\repo\MyFlowHub-Server\docs\specs\varstore.md`
  - `D:\project\MyFlowHub3\repo\MyFlowHub-Server\docs\specs\topicbus.md`
- Active execution control document is this worktree-root `plan.md`, per `$m-autoflow` root-plan exception.
- Workflow result archive will be created later under worktree `docs/change/YYYY-MM-DD_sdk-typed-clients.md`.
- No reusable lesson is currently required; reassess in Stage 4 if dependency drift or target-id pitfalls need a lessons entry.

#### Related Requirements / Specs / Lessons
- Requirements impact: `none`
  - No SDK-specific stable requirements doc exists in this repo.
  - The work implements the already documented SDK README direction: typed subprotocol clients as a future small-step evolution.
- Specs impact: `none`
  - No protocol wire contract changes are planned.
  - Implementation must follow existing specs instead of editing them.
- Related requirements:
  - `D:\project\MyFlowHub3\worktrees\MyFlowHub-SDK-feat-sdk-typed-clients\README.md`
- Related specs:
  - `D:\project\MyFlowHub3\repo\MyFlowHub-Server\docs\specs\core.md`
  - `D:\project\MyFlowHub3\repo\MyFlowHub-Server\docs\specs\protocol_map.md`
  - `D:\project\MyFlowHub3\repo\MyFlowHub-Server\docs\specs\auth.md`
  - `D:\project\MyFlowHub3\repo\MyFlowHub-Server\docs\specs\varstore.md`
  - `D:\project\MyFlowHub3\repo\MyFlowHub-Server\docs\specs\topicbus.md`
- Related lessons:
  - none currently
- Related historical SDK change docs:
  - `docs/change/2026-02-16_sdk-v1-await.md`
  - `docs/change/2026-03-06_varstore-cmd-await-sdk.md`
  - `docs/change/2026-03-15_sdk-await-send-guard.md`
  - `docs/change/2026-03-16_sdk-quic-endpoint.md`

#### Executable Task List
- T1 - Add shared typed client foundation.
- T2 - Add Management typed client.
- T3 - Add Auth admin typed client.
- T4 - Add VarStore typed client.
- T5 - Add TopicBus typed client.
- T6 - Align dependencies and documentation.
- T7 - Validate, review, and archive.

#### Task Details

##### T1 - Shared typed client foundation
- Owner: Main Agent
- Worktree: `D:\project\MyFlowHub3\worktrees\MyFlowHub-SDK-feat-sdk-typed-clients`
- Plan Path: `D:\project\MyFlowHub3\worktrees\MyFlowHub-SDK-feat-sdk-typed-clients\plan.md`
- Goal: Provide a small shared layer for options, header construction, send-and-decode, send-only, and validation helpers.
- Files / Modules:
  - new typed client package files, exact paths to be chosen before edit summary in Stage 3.2
- Write Set:
  - new SDK typed package only
- Acceptance:
  - helper rejects nil client, missing source/target for non-Auth control plane, empty expected response action, and decode errors.
  - helper builds `MajorCmd` headers with correct SubProto/SourceID/TargetID.
- Test Points:
  - focused helper tests or subclient tests covering helper behavior.
- Rollback:
  - delete new typed package files.

##### T2 - Management typed client
- Owner: Main Agent
- Worktree: `D:\project\MyFlowHub3\worktrees\MyFlowHub-SDK-feat-sdk-typed-clients`
- Plan Path: `D:\project\MyFlowHub3\worktrees\MyFlowHub-SDK-feat-sdk-typed-clients\plan.md`
- Goal: Wrap management actions from Proto and protocol_map notes.
- Files / Modules:
  - new management typed client files
  - typed client tests
- Write Set:
  - new SDK typed package files and tests
- Acceptance:
  - methods: `NodeEcho`, `NodeInfo`, `ListNodes`, `ListSubtree`, `ConfigGet`, `ConfigList`, `ConfigSet`.
  - `ConfigGet/ConfigSet` reject empty key.
  - fake server observes SubProto=1 and expected actions.
- Test Points:
  - success response decode for at least `NodeEcho` and one list/config method.
  - validation failure for empty config key.
- Rollback:
  - remove management methods/tests.

##### T3 - Auth admin typed client
- Owner: Main Agent
- Worktree: `D:\project\MyFlowHub3\worktrees\MyFlowHub-SDK-feat-sdk-typed-clients`
- Plan Path: `D:\project\MyFlowHub3\worktrees\MyFlowHub-SDK-feat-sdk-typed-clients\plan.md`
- Goal: Wrap Auth authority/admin actions without implementing login/register signing.
- Files / Modules:
  - new auth typed client files
  - typed client tests
- Write Set:
  - new SDK typed package files and tests
- Acceptance:
  - methods: `GetPerms`, `ListRoles`, `ListPendingRegisters`, `ListRegisterPermits`, `ApproveRegister`, `RejectRegister`, `IssueRegisterPermit`, `RevokeRegisterPermit`.
  - `ListRoles` uses an SDK-local response struct matching `auth.md` because Proto currently exposes `RolePermEntry` and `ListRolesReq`, but no dedicated `ListRolesResp`.
  - request IDs, device IDs, roles, and permit tokens are validated where required.
  - docs say full signed register/login is deferred.
- Test Points:
  - success decode for `GetPerms` and one permit/approval method.
  - validation failures for empty request id / permit / device id / role.
- Rollback:
  - remove auth admin methods/tests.

##### T4 - VarStore typed client
- Owner: Main Agent
- Worktree: `D:\project\MyFlowHub3\worktrees\MyFlowHub-SDK-feat-sdk-typed-clients`
- Plan Path: `D:\project\MyFlowHub3\worktrees\MyFlowHub-SDK-feat-sdk-typed-clients\plan.md`
- Goal: Wrap VarStore request-response actions while preserving MajorCmd response support.
- Files / Modules:
  - new varstore typed client files
  - typed client tests
- Write Set:
  - new SDK typed package files and tests
- Acceptance:
  - methods: `Set`, `Get`, `List`, `Revoke`, `Subscribe`, `Unsubscribe`.
  - `Unsubscribe` is send-only because the current Proto/spec has no stable `unsubscribe_resp` action; existing asynchronous frames remain caller-managed through `onUnmatched`.
  - `name` validates as letters/digits/underscore.
  - `Set` rejects blank value and normalizes empty visibility to `public`.
  - `Subscribe` rejects subscriber not 0 and not SourceID.
  - fake server can respond with `MajorCmd` and typed client decodes it.
- Test Points:
  - success decode for `Set` or `Get` using `MajorCmd` response.
  - validation failures for invalid name, blank value, invalid visibility, invalid subscriber.
- Rollback:
  - remove varstore methods/tests.

##### T5 - TopicBus typed client
- Owner: Main Agent
- Worktree: `D:\project\MyFlowHub3\worktrees\MyFlowHub-SDK-feat-sdk-typed-clients`
- Plan Path: `D:\project\MyFlowHub3\worktrees\MyFlowHub-SDK-feat-sdk-typed-clients\plan.md`
- Goal: Wrap TopicBus subscribe/list request-response methods and no-ack publish.
- Files / Modules:
  - new topicbus typed client files
  - typed client tests
- Write Set:
  - new SDK typed package files and tests
- Acceptance:
  - methods: `Subscribe`, `SubscribeBatch`, `Unsubscribe`, `UnsubscribeBatch`, `ListSubs`, `Publish`.
  - topic strings are sent unchanged.
  - `Publish` rejects blank event name and does not wait for response.
- Test Points:
  - fake server observes raw topic string unchanged.
  - publish returns after send without server response.
  - batch methods decode response.
- Rollback:
  - remove topicbus methods/tests.

##### T6 - Align dependencies and documentation
- Owner: Main Agent
- Worktree: `D:\project\MyFlowHub3\worktrees\MyFlowHub-SDK-feat-sdk-typed-clients`
- Plan Path: `D:\project\MyFlowHub3\worktrees\MyFlowHub-SDK-feat-sdk-typed-clients\plan.md`
- Goal: Keep SDK release-mode dependency versions aligned with currently referenced Proto/Core and document the API.
- Files / Modules:
  - `go.mod`
  - `go.sum`
  - `README.md`
- Write Set:
  - dependency metadata and README only
- Acceptance:
  - `myflowhub-core` aligns to `v0.4.10` if required by current SDK release checks.
  - `myflowhub-proto` aligns to `v0.1.7` if required by current Server/Proto matrix.
  - README includes typed client example and target-id warning.
- Test Points:
  - `GOWORK=off go test ./... -count=1 -p 1`.
- Rollback:
  - revert dependency and README edits.

##### T7 - Validate, review, and archive
- Owner: Main Agent
- Worktree: `D:\project\MyFlowHub3\worktrees\MyFlowHub-SDK-feat-sdk-typed-clients`
- Plan Path: `D:\project\MyFlowHub3\worktrees\MyFlowHub-SDK-feat-sdk-typed-clients\plan.md`
- Goal: Complete Stage 3.3 code review and Stage 4 change archive after implementation.
- Files / Modules:
  - `docs/change/2026-06-07_sdk-typed-clients.md` or current-date equivalent
  - possible `docs/lessons/*` only if Stage 4 identifies reusable lesson value
- Write Set:
  - workflow archive docs
- Acceptance:
  - Stage 3.3 checklist marks every item pass or returns to implementation.
  - `$m-docs` Stage 4 impact checks recorded.
  - Change archive includes plan task mapping, verification results, rollback, and subagent trace.
- Test Points:
  - `git diff --check`
  - `GOWORK=off go test ./... -count=1 -p 1`
- Rollback:
  - archive docs can be reverted with the workflow commit.

#### Dependencies
- Core header package and `await.Client` behavior.
- Proto packages:
  - `github.com/yttydcs/myflowhub-proto/protocol/management`
  - `github.com/yttydcs/myflowhub-proto/protocol/auth`
  - `github.com/yttydcs/myflowhub-proto/protocol/varstore`
  - `github.com/yttydcs/myflowhub-proto/protocol/topicbus`
- Server specs are read-only source of truth for behavior.

#### Risks and Notes
- Keep package naming conservative; avoid generic names that conflict with existing `transport`/`session`/`await`.
- Do not alter `await.Client.handleFrame` unless tests reveal a specific typed-client blocker.
- Do not normalize TopicBus topics.
- Do not introduce global default target behavior.
- Protocol response `code` remains caller-visible.

#### Parallelism Assessment
- Parallel implementation is theoretically possible by subprotocol package, but this workflow should stay single-agent because:
  - API shape and helper package are shared across all tasks.
  - File write set is small.
  - Consistency of validation and docs matters more than raw parallelism.
- SubAgent use: none planned.
- Owner: Main Agent
- Write set: active worktree only.

#### Issue List
- User confirmed this `plan.md` by saying `请继续`.

阻塞：否
进入 3.2
不派发子Agent：共享 helper/API 写集耦合较高，由 Main Agent 单线程实施并集成。

### Stage 3.2 - Implementation Summary

- T1 completed: added shared `typed` client foundation.
- T2 completed: added Management typed client.
- T3 completed: added Auth admin typed client.
- T4 completed: added VarStore typed client.
- T5 completed: added TopicBus typed client.
- T6 completed: upgraded Core/Proto dependencies and updated README.

Implementation files:
- `typed/client.go`
- `typed/management.go`
- `typed/auth.go`
- `typed/varstore.go`
- `typed/topicbus.go`
- `typed/client_test.go`
- `README.md`
- `go.mod`
- `go.sum`

Validation:
- `$env:GOWORK='off'; go test ./... -count=1 -p 1` passed.
- `git diff --check` passed.

### Stage 3.3 - Code Review

- 需求覆盖：通过
- 架构合理性：通过
- 性能风险（N+1 / 重复计算 / 多余 I/O / 锁竞争）：通过
- 可读性与一致性：通过
- 可扩展性与配置化：通过
- 稳定性与安全：通过
- 测试覆盖情况：通过
- 子Agent治理与审计：通过（未派发；共享 helper/API 写集耦合高）

### Stage 4 - Change Archive

- `$m-docs` used for archive routing and requirements/specs/lessons impact checks.
- Archive path: `docs/change/2026-06-07_sdk-typed-clients.md`
- Requirements impact: `none`
- Specs impact: `none`
- Lessons impact: `none`

## Iteration 2 - Auth register/login ES256 helpers

### Rollback Reason
- User requested complete and sufficient Auth register/login ES256 helper support after the first typed-client set was archived but not closed.
- The previous plan explicitly deferred Auth register/login signing, key storage, and nonce helpers, so this is a scope expansion and must roll back from Stage 4 to Stage 1 / Stage 2 / Stage 3.1 before implementation.
- No source code edits for this expanded scope may happen until this Stage 3.1 plan is confirmed.

### Stage 1 - Requirements Analysis

#### Goal
Implement SDK-level Auth register/login helpers that let Win, Android, CLI, and other Go clients perform first registration and signed login without reimplementing P256 key generation, DER/base64 encoding, ES256 signing, nonce generation, Auth JSON envelope construction, and bootstrap routing.

#### Scope
Must:
- Add Auth key/signing primitives in `MyFlowHub-SDK` using standard-library crypto only.
- Match the current Auth spec exactly:
  - SubProto = `2`
  - envelope = `{"action":"<name>","data":{...}}`
  - key format = base64 DER
  - public key DER = PKIX public key
  - private key DER = EC private key
  - algorithm = `ES256`
  - login digest bytes = `login\n<trim(device_id)>\n<node_id>\n<ts>\n<trim(nonce)>`
  - signature = SHA256 digest signed with ECDSA ASN.1, then base64 encoded
- Add typed `Auth().Register` and `Auth().Login` helpers returning `auth.RespData`.
- Support unauthenticated bootstrap calls where `SourceID=0` and `TargetID=0` are valid only for direct Auth register/login.
- Preserve the existing strict non-zero route validation for authenticated Auth admin/control methods.
- Validate required fields and fail before sending on invalid local input.
- Add tests that verify key format, sign bytes, signature verification, wire action/header/data, bootstrap route, and validation failures.
- Update README and Stage 4 change archive after implementation.

May:
- Add file persistence helpers for the common `config/node_keys.json` shape used by current Android and Server docs: `{"privkey":"<base64 DER>","pubkey":"<base64 DER>"}`.
- Add request builders that fill `ts`, `nonce`, `alg`, `sig`, `pubkey`, and `node_pub` safely while still allowing advanced callers to pass explicit values for deterministic tests.
- Allow explicit route override for Auth register/login when a caller already knows a non-zero authority route.

Will not:
- Change Server, SubProto, Core, or Proto protocol behavior.
- Add new wire fields not present in `auth.RegisterData` / `auth.LoginData`.
- Implement `assist_register`, `assist_login`, `up_login`, `assist_query_credential`, `revoke`, `offline`, or authority policy management in this iteration.
- Convert Auth `data.code` values into Go errors; local validation/transport/decode errors are Go errors, protocol results remain in `auth.RespData`.
- Add OS-specific secret storage or platform keystore integration in the SDK.
- Migrate Win or Android call sites in this SDK workflow.

#### Use Cases
- A new desktop/mobile/CLI client generates or loads a node key pair, calls `Auth().Register`, receives `approved`, `pending`, or `rejected`, and can retry after approval without hand-built envelopes.
- A registered client loads its private key and calls `Auth().Login`; the SDK fills timestamp, nonce, signature, and `alg=ES256`.
- Tests and diagnostic tools can create deterministic login requests by supplying fixed timestamp/nonce values and verifying the exact signature contract.
- Advanced deployments can still target a known remote authority route, while the common direct bootstrap path defaults to `SourceID=0`, `TargetID=0`.

#### Functional Requirements
- Generate P256 ECDSA keys and encode them as base64 DER matching SubProto/Android behavior.
- Parse base64 DER private keys and public keys, rejecting non-P256 or malformed keys.
- Generate cryptographically random hex nonces, with a safe default byte length.
- Expose the exact login signing bytes helper for tests and cross-app compatibility checks.
- Sign login requests with SHA256 + ECDSA ASN.1 and base64 signature output.
- `Register` must require non-empty `device_id` and a valid public key, either supplied explicitly or derived from an SDK key pair/helper.
- `Register` must set `pubkey` and `node_pub` consistently when it derives the public key.
- `Login` must require non-empty `device_id`, `node_id > 0`, valid private key or explicit signature inputs, timestamp, nonce, and `alg=ES256`.
- `Login` must not include `display_name` in the signing bytes.
- Bootstrap register/login must build `MajorCmd`, SubProto Auth, default `SourceID=0`, `TargetID=0`, and expected response action `register_resp` / `login_resp`.
- Auth admin/control methods already implemented in T3 must continue requiring real non-zero SourceID/TargetID.

#### Non-functional Requirements
- SDK dependency direction remains Core + Proto + standard library only.
- No silent crypto failures: invalid keys, random nonce failures, file parse/write failures, nil client, nil key, invalid route, and decode failures must be explicit errors.
- Key persistence helper must avoid unnecessary writes: load valid existing keys first and only create when missing/invalid according to the selected API semantics.
- Private key files written by SDK helper should use restrictive permissions where supported by the platform.
- Public API must remain small, explicit, and stable enough for Win/Android/CLI reuse.
- Tests must run with `GOWORK=off` to prove release-mode dependency consistency.

#### Inputs / Outputs
- Inputs:
  - connected `*await.Client`
  - optional typed client route defaults
  - `device_id`, `node_id`, optional `display_name`
  - optional requested role / join permit for register
  - private key/public key or key pair helper output
  - optional timestamp and nonce for deterministic callers
- Outputs:
  - `auth.RespData` for register/login protocol responses
  - base64 DER public/private key strings
  - base64 ES256 signatures
  - Go errors for local validation, crypto, I/O, send/await, or decode failures

#### Edge Cases
- `SourceID=0` is only acceptable for Auth bootstrap register/login; it must not weaken route checks for management, varstore, topicbus, or Auth admin actions.
- `TargetID=0` is normally child broadcast; for direct unauthenticated Auth register/login it matches the existing Android/bootstrap path and Core allowlist behavior.
- Register does not currently require a signature; the SDK must not invent one.
- Register may return `code=202,status=pending` and no `node_id`; this is a successful transport/protocol response, not a Go error.
- Approve only reserves identity; the client must retry register to complete admission.
- Login requires a real assigned `node_id`; `node_id=0` must fail locally.
- Whitespace in `device_id` and `nonce` is trimmed in sign bytes; API should avoid signing one value and sending a materially different value.
- Random nonce generation failure is unlikely but must not be swallowed.
- Existing malformed key files must fail explicitly or be regenerated only by an API whose name clearly states load-or-create semantics.

#### Acceptance Criteria
- Unit tests pass for:
  - key generation and parse round trip
  - private/public DER base64 shape
  - exact `LoginSignBytes` output for a fixed sample
  - `SignLogin` output verifies with the generated public key using SHA256 + ECDSA ASN.1
  - register sends Auth SubProto `2`, action `register`, default route `SourceID=0`, `TargetID=0`, and matching `pubkey/node_pub`
  - login sends Auth SubProto `2`, action `login`, default route `SourceID=0`, `TargetID=0`, `alg=ES256`, fixed `ts/nonce`, and a verifiable signature
  - validation failures for missing device id, missing node id, nil/invalid keys, invalid nonce size, and malformed key file content
- README documents Auth register/login helper usage and the remaining boundaries.
- `GOWORK=off go test ./... -count=1 -p 1` passes.
- `git diff --check` passes.
- Stage 3.3 code review passes all required checks before Stage 4 archive is updated.

#### Risks
- Crypto helper compatibility is security-sensitive; the implementation must be byte-for-byte aligned with SubProto and Android signing logic.
- A generic route relaxation could accidentally permit invalid unauthenticated calls outside Auth bootstrap; route bypass must be method-scoped.
- Key file helper can create false security expectations; document that platform keystore integration remains host responsibility.
- API overloads can become confusing; prefer clear request structs over many positional parameters.
- README must remove the old statement that Auth register/login is not covered once implementation is complete.

#### Issue List
- None currently blocking Stage 3.1 plan confirmation.

### Stage 2 - Architecture Design

#### Overall Solution
Extend the existing `typed` package with a focused Auth bootstrap/signing layer. Keep the current Auth admin client unchanged, add crypto/key helpers in separate files, and add register/login request helpers that use a dedicated Auth bootstrap send-and-decode path instead of weakening the shared `validateRoute`.

Selection rationale:
- `await.Client` already provides MsgID generation, context cancellation, response matching, and TCP framing.
- `auth.RegisterData`, `auth.LoginData`, and `auth.RespData` already exist in Proto, so no SDK-local wire structs are needed for register/login.
- SubProto and Android already prove the ES256 key/signature contract; duplicating that logic once in SDK removes app-level drift.
- Keeping bootstrap route handling inside Auth methods preserves the existing safety checks for all other typed clients.

#### Alternatives Considered
- Keep Auth register/login in each app: rejected because it has already drifted into Android-specific helper code and would force Win/Electron/Flutter hosts to repeat crypto details.
- Import SubProto auth crypto helpers: rejected because SDK must not depend on SubProto/Server and those helpers are not a client API.
- Make `validateRoute` accept `SourceID=0/TargetID=0` for all Auth: rejected because admin/control Auth methods require authenticated real node routing.
- Implement platform keystore integration now: rejected because SDK is Go-only and cross-platform storage policy belongs to host apps.

#### Module Responsibilities
- `typed/client.go`:
  - keep strict route validation for normal typed clients
  - add or support an internal method-scoped send helper for explicit Auth register/login routes
- `typed/auth.go`:
  - retain Auth admin/control methods
  - add `Register` and `Login` methods that build Proto request data and decode `auth.RespData`
- new Auth crypto/key file:
  - key pair type
  - generate/parse/encode helpers
  - nonce generation
  - login sign bytes and ES256 signing helper
  - optional load-or-create JSON key-file helper
- tests:
  - extend fake TCP tests for Auth register/login bootstrap wire behavior
  - add crypto/key tests with deterministic timestamp/nonce and real signature verification
- README/docs:
  - document helper usage and route/security boundaries

#### Data / Call Flow
Register:
1. Caller creates/loads an Auth key pair.
2. Caller calls `tc.Auth().Register(ctx, req)` or equivalent helper request.
3. SDK validates `device_id` and public key.
4. SDK fills `pubkey` and `node_pub` when derived from the key pair.
5. SDK sends `{"action":"register","data":auth.RegisterData{...}}` with Auth SubProto `2`.
6. SDK decodes `register_resp` into `auth.RespData`; `code/status` remains caller-visible.

Login:
1. Caller loads private key and knows assigned `node_id`.
2. Caller calls `tc.Auth().Login(ctx, req)` with `device_id`, `node_id`, optional `display_name`, optional fixed `ts/nonce`.
3. SDK fills missing `ts`, `nonce`, `alg=ES256`, and `sig`.
4. Signing bytes are exactly `login\n<device_id>\n<node_id>\n<ts>\n<nonce>` after the same trimming rules used by SubProto.
5. SDK sends `{"action":"login","data":auth.LoginData{...}}` with Auth SubProto `2`.
6. SDK decodes `login_resp` into `auth.RespData`.

#### Interface Drafts
Exact names may be refined during implementation, but the shape should stay request-struct based:

```go
base := await.NewClient(ctx, onUnmatched, onError)
_ = base.Connect("127.0.0.1:9000")

tc := typed.New(base, typed.Options{})

keys, err := typed.LoadOrCreateAuthKeyPair("config/node_keys.json")
resp, err := tc.Auth().Register(ctx, typed.AuthRegisterRequest{
    DeviceID:    "win-001",
    DisplayName: "Win node",
    KeyPair:     keys,
})

login, err := tc.Auth().Login(ctx, typed.AuthLoginRequest{
    DeviceID: "win-001",
    NodeID:   resp.NodeID,
    KeyPair:  keys,
})
```

Lower-level helpers should remain available for host apps that manage keys themselves:

```go
keys, err := typed.GenerateAuthKeyPair()
sig, err := typed.SignLogin(keys.PrivateKey, "win-001", 5, 1700000000, "n1")
msg := typed.LoginSignBytes("win-001", 5, 1700000000, "n1")
```

#### Error Handling and Safety
- Return Go errors for:
  - nil typed/await client
  - invalid route override
  - missing `device_id`
  - missing public key for register
  - missing `node_id` for login
  - nil private key for signing
  - malformed/non-P256 keys
  - invalid nonce byte length
  - random source failures
  - key file JSON/parse/write failures
  - send/await/decode failures
- Do not return Go errors for protocol business outcomes such as pending approval, invalid signature from authority, permission denied, or authority unavailable; those remain `auth.RespData.Code`.
- Do not log or expose private key material in errors or README examples.
- Keep private key file permission restrictive on write; document that host apps may wrap with platform keystores.

#### Performance and Testing Strategy
- Crypto operations are O(1) per request and use standard-library P256/ECDSA.
- Avoid repeated file I/O by leaving caching to host apps; `LoadOrCreateAuthKeyPair` performs one load/create operation per call.
- Tests use fake TCP servers to assert actual wire headers and payloads instead of only checking internal request builders.
- Crypto tests verify signatures with standard library rather than trusting SDK signing output.
- Final validation:
  - `gofmt -w typed`
  - `$env:GOWORK='off'; go test ./... -count=1 -p 1`
  - `git diff --check`

#### Extensibility Design Points
- Register/Login request structs can later grow route override fields without affecting Auth admin APIs.
- Key helper file format matches existing Server/Android `node_keys.json`, so host migrations can share files.
- Separate low-level crypto helpers from high-level Auth methods so future non-Go hosts can port the exact contract from tests/docs.
- Future `assist_*` and `up_login` helpers should be planned separately because they have multi-hop authority and sender-signature semantics.

#### Issue List
- None currently blocking Stage 3.1 plan confirmation.

### Stage 3.1 - Planning

#### Project Goal and Current State
- Current worktree already contains the first typed client layer, but Auth register/login was explicitly deferred.
- Current gap: apps can call Auth admin/control helpers, but still need to reimplement first registration, key generation, nonce, and signed login.
- This iteration completes the client-side Auth bootstrap path without changing server-side protocol behavior.

#### Docs Governance Routing Decision
- Used `$m-docs` for plan routing and impact checks.
- SDK docs tree currently has `docs/change` and `docs/plan_archive`; it has no stable `docs/requirements` or `docs/specs` tree to update.
- Stable Auth technical contract remains in:
  - `D:\project\MyFlowHub3\repo\MyFlowHub-Server\docs\specs\auth.md`
  - `D:\project\MyFlowHub3\repo\MyFlowHub-Proto\protocol\auth\types.go`
  - `D:\project\MyFlowHub3\repo\MyFlowHub-SubProto\auth\crypto.go`
  - `D:\project\MyFlowHub3\repo\MyFlowHub-SubProto\auth\actions_register.go`
  - `D:\project\MyFlowHub3\repo\MyFlowHub-SubProto\auth\actions_login.go`
  - `D:\project\MyFlowHub3\repo\MyFlowHub-Android\hubmobile\keys.go`
- Active execution control document remains this worktree-root `plan.md`, per `$m-autoflow` root-plan exception.
- Workflow result archive will be updated or supplemented under worktree `docs/change` during Stage 4.
- No reusable `docs/lessons` entry is required yet; reassess during Stage 4 if the route exception or ES256 contract needs a lookup lesson.

#### Related Requirements / Specs / Lessons
- Requirements impact: `none`
  - No SDK-specific stable requirements doc exists in this repo.
  - README will be updated as product-facing SDK capability documentation after implementation.
- Specs impact: `none`
  - No Auth wire/protocol contract changes are planned.
  - Implementation must follow the current Auth spec and existing SubProto/Android signing behavior.
- Related requirements:
  - `D:\project\MyFlowHub3\worktrees\MyFlowHub-SDK-feat-sdk-typed-clients\README.md`
- Related specs:
  - `D:\project\MyFlowHub3\repo\MyFlowHub-Server\docs\specs\auth.md`
  - `D:\project\MyFlowHub3\repo\MyFlowHub-Proto\protocol\auth\types.go`
  - `D:\project\MyFlowHub3\repo\MyFlowHub-SubProto\auth\crypto.go`
  - `D:\project\MyFlowHub3\repo\MyFlowHub-SubProto\auth\actions_register.go`
  - `D:\project\MyFlowHub3\repo\MyFlowHub-SubProto\auth\actions_login.go`
  - `D:\project\MyFlowHub3\repo\MyFlowHub-Android\hubmobile\keys.go`
- Related lessons:
  - none currently

#### Executable Task List
- T8 - Add Auth key and ES256 signing primitives.
- T9 - Add Auth register/login typed helpers with scoped bootstrap routing.
- T10 - Add Auth crypto/wire tests and update README.
- T11 - Re-run validation, Stage 3.3 review, and Stage 4 archive update.

#### Task Details

##### T8 - Add Auth key and ES256 signing primitives
- Owner: Main Agent
- Worktree: `D:\project\MyFlowHub3\worktrees\MyFlowHub-SDK-feat-sdk-typed-clients`
- Plan Path: `D:\project\MyFlowHub3\worktrees\MyFlowHub-SDK-feat-sdk-typed-clients\plan.md`
- Goal: Provide reusable SDK primitives for P256 key generation, base64 DER encode/decode, nonce generation, login sign bytes, and ES256 signatures.
- Files / Modules:
  - `typed/auth_keys.go` or equivalent new file
  - `typed/client_test.go` or new focused auth test file
- Write Set:
  - `typed/*`
- Acceptance:
  - Generated public/private keys parse as P256.
  - Encoded formats match Server/Android `node_keys.json`: `privkey`, `pubkey`, base64 DER.
  - `LoginSignBytes` matches SubProto/Android exact string.
  - `SignLogin` signatures verify using the generated public key.
  - Nonce helper returns hex output and rejects invalid byte lengths when applicable.
- Test Points:
  - key round trip
  - malformed base64 and non-P256 rejection
  - exact sign bytes sample
  - standard-library signature verification
- Rollback:
  - delete the new key/signing helper file and related tests.

##### T9 - Add Auth register/login typed helpers with scoped bootstrap routing
- Owner: Main Agent
- Worktree: `D:\project\MyFlowHub3\worktrees\MyFlowHub-SDK-feat-sdk-typed-clients`
- Plan Path: `D:\project\MyFlowHub3\worktrees\MyFlowHub-SDK-feat-sdk-typed-clients\plan.md`
- Goal: Add high-level `Auth().Register` and `Auth().Login` methods that construct Proto requests, sign login, and send via Auth SubProto without weakening other typed route validation.
- Files / Modules:
  - `typed/auth.go`
  - `typed/client.go` only if an internal route-specific helper is needed
  - auth tests
- Write Set:
  - `typed/*`
- Acceptance:
  - `Register` sends `auth.ActionRegister` and decodes `auth.ActionRegisterResp`.
  - `Login` sends `auth.ActionLogin` and decodes `auth.ActionLoginResp`.
  - Default register/login route is `SourceID=0`, `TargetID=0`.
  - Optional explicit route override can use non-zero `SourceID/TargetID` for advanced callers.
  - Auth admin methods continue using strict non-zero route validation.
  - `Register` fills `pubkey/node_pub` consistently when deriving from a key pair.
  - `Login` fills `ts`, `nonce`, `sig`, and `alg=ES256` when absent.
- Test Points:
  - fake server asserts register header/action/data and returns approved/pending response.
  - fake server asserts login header/action/data and verifies signature.
  - validation failures for missing device id, node id, key, invalid route override.
- Rollback:
  - remove register/login methods and any internal scoped route helper.

##### T10 - Add Auth crypto/wire tests and update README
- Owner: Main Agent
- Worktree: `D:\project\MyFlowHub3\worktrees\MyFlowHub-SDK-feat-sdk-typed-clients`
- Plan Path: `D:\project\MyFlowHub3\worktrees\MyFlowHub-SDK-feat-sdk-typed-clients\plan.md`
- Goal: Lock the Auth helper behavior with tests and document the supported SDK surface.
- Files / Modules:
  - `typed/*_test.go`
  - `README.md`
- Write Set:
  - tests and README only
- Acceptance:
  - README no longer says Auth register/login is out of scope.
  - README includes register/login example with key generation/load and route warning.
  - Tests cover both crypto primitives and actual TCP wire behavior.
- Test Points:
  - `go test ./typed -count=1`
  - full repository test in T11
- Rollback:
  - revert README/test additions.

##### T11 - Re-run validation, review, and archive
- Owner: Main Agent
- Worktree: `D:\project\MyFlowHub3\worktrees\MyFlowHub-SDK-feat-sdk-typed-clients`
- Plan Path: `D:\project\MyFlowHub3\worktrees\MyFlowHub-SDK-feat-sdk-typed-clients\plan.md`
- Goal: Complete validation, mandatory Stage 3.3 review, and Stage 4 docs archive after implementation.
- Files / Modules:
  - `docs/change/2026-06-07_sdk-typed-clients.md` and/or `docs/change/2026-06-08_sdk-auth-signing-helpers.md`
  - possible `docs/lessons/*` only if Stage 4 identifies reusable lesson value
- Write Set:
  - workflow archive docs
- Acceptance:
  - `gofmt` completed for touched Go files.
  - `$env:GOWORK='off'; go test ./... -count=1 -p 1` passes.
  - `git diff --check` passes.
  - Stage 3.3 checklist passes or loops back to T8/T9/T10.
  - Stage 4 archive records requirements/specs/lessons impact and task mapping.
- Test Points:
  - full test and whitespace validation commands above.
- Rollback:
  - revert implementation and archive docs from this iteration.

#### Dependencies
- `await.Client` for Send/SendAndAwait and MsgID matching.
- `transport.EncodeMessage` / `DecodeMessage` envelope behavior.
- Core `HeaderTcp` and Auth SubProto routing behavior.
- Proto Auth action constants and data structs.
- Standard library packages: `crypto/ecdsa`, `crypto/elliptic`, `crypto/rand`, `crypto/sha256`, `crypto/x509`, `encoding/base64`, `encoding/hex`, `encoding/json`, `os`, `path/filepath`, `strings`, `time`.

#### Risks and Notes
- Keep the route exception local to register/login.
- Keep `display_name` out of login signing bytes.
- Do not trim TopicBus topics or change existing typed client semantics while touching shared helpers.
- Avoid adding dependencies for crypto or file handling.
- Preserve existing uncommitted work in this worktree; do not reset or revert unrelated files.

#### Parallelism Assessment
- Parallel implementation remains possible in theory between crypto helpers and README/tests, but this iteration should stay single-agent because:
  - the Auth public API shape affects tests and README directly.
  - route helper changes can affect existing typed clients.
  - crypto and wire behavior require tight cross-checking against specs.
- SubAgent use: none planned.
- Owner: Main Agent.
- Write set: active worktree only.

#### Issue List
- User confirmed this expanded Stage 3.1 plan by saying `请继续`.

阻塞：否
进入 3.2
不派发子Agent：Auth public API, scoped bootstrap routing, crypto helpers, tests, and README form one compatibility surface and remain with Main Agent.

### Stage 3.2 - Implementation Summary

- T8 completed: added SDK Auth key and ES256 signing primitives.
- T9 completed: added Auth register/login typed helpers with scoped bootstrap routing.
- T10 completed: added crypto/wire tests and README updates.
- T11 validation completed; Stage 3.3 and Stage 4 records below.

Implementation files:
- `typed/auth_keys.go`
- `typed/auth.go`
- `typed/client.go`
- `typed/auth_keys_test.go`
- `typed/auth_wire_test.go`
- `README.md`

Design notes:
- `Auth().Register` and `Auth().Login` default to the direct unauthenticated Auth bootstrap route `SourceID=0, TargetID=0`.
- Existing Auth admin/control methods continue to use strict `SourceID!=0 && TargetID!=0` validation through the existing `sendAndDecode` path.
- ES256 helpers use P256, SHA256, ASN.1 ECDSA signatures, and base64 DER private/public key encodings matching Server/SubProto/Win/Android.
- `LoadOrCreateAuthKeyPair` creates a file only when missing; malformed existing key files fail explicitly and are not overwritten.
- Protocol `data.code` remains caller-visible in `auth.RespData`; local validation, crypto, I/O, send/await, and decode failures return Go errors.

Validation:
- `$env:GOWORK='off'; go test ./typed -count=1` passed.
- `$env:GOWORK='off'; go test ./... -count=1 -p 1` passed.
- `git diff --check` passed.

### Stage 3.3 - Code Review

- 需求覆盖：通过
  - T8/T9/T10/T11 acceptance points are covered by implementation and tests.
- 架构合理性：通过
  - SDK remains a thin layer over `await.Client`; no Server/SubProto dependency was introduced.
  - Auth bootstrap route exception is method-scoped to register/login.
- 性能风险（N+1 / 重复计算 / 多余 I/O / 锁竞争）：通过
  - Crypto work is per-call O(1); key-file helper performs one load/create operation per call and leaves caching to hosts.
- 可读性与一致性：通过
  - Public API uses explicit request structs and follows existing typed package patterns.
- 可扩展性与配置化：通过
  - `Route *Options` allows advanced authority routing without weakening normal typed defaults.
  - Key/signing helpers are separate from high-level Auth methods.
- 稳定性与安全：通过
  - Invalid keys, mismatched key pairs, unsupported alg, missing node/device fields, bad route override, bad key file, and nonce errors fail explicitly.
  - Existing malformed key files are not silently replaced.
- 测试覆盖情况：通过
  - Added key round-trip, sign bytes, signature verification, key file behavior, validation errors, register wire, login wire, and route-guard tests.
- 子Agent治理与审计：通过
  - No subAgents used; coupling and write-set rationale recorded.

### Stage 4 - Change Archive

- `$m-docs` used for archive routing and requirements/specs/lessons impact checks.
- Archive path: `docs/change/2026-06-08_sdk-auth-signing-helpers.md`
- Requirements impact: `none`
- Specs impact: `none`
- Lessons impact: `none`
