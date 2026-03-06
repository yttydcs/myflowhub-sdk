# 2026-03-06 await：兼容 VarStore `MajorCmd` 回程响应（SDK）

## 变更规模
- 级别：较大（协议回程 Major 语义调整带来的联动，需避免误匹配吞帧）

## 背景 / 目标
VarStore 的 `*_resp/assist_*_resp` 已按规范调整为 `Header.Major=MajorCmd`（逐跳可见），以便中间节点能够处理响应并完成：

- 去重扇出
- 沿途缓存更新
- 并发写入 1:1 匹配（基于 msg_id 映射）

但 SDK `await.Client` 旧实现仅接受 `MajorOKResp/MajorErrResp`，导致 VarStore 响应帧会被当作 unmatched，最终 `SendAndAwait` 超时。

目标：**在不放宽全局 Major 过滤的前提下**，为 VarStore 增加一个严格白名单兼容路径。

## 变更内容
- `await.Client.handleFrame`：
  - 仍保持默认只接受 `MajorOKResp/MajorErrResp`；
  - 新增例外：当 `SubProto==VarStore(SubProto=3)` 且 `Major==MajorCmd` 时允许进入 decode+deliver；
  - 其它子协议仍拒绝 `MajorCmd`，避免普通 Cmd 帧误匹配被吞掉。

## 影响范围
- 仅影响 `await` 的匹配入口，不改变 `Broker` Key 设计（仍按 `MsgID+SubProto+Action`）。
- 对其它子协议保持严格策略，降低误匹配风险。

## 关联任务映射（plan.md）
- SDK-1：await 匹配策略扩展（VarStore MajorCmd 白名单）
- SDK-2：回归兼容与测试（新增用例 + 旧用例不回归）

## 测试与验证
- 单测：
  - `cd .; $env:GOWORK='off'; go test ./... -count=1`
- 新增覆盖点：
  - VarStore `MajorCmd` 响应可被 deliver
  - 非 VarStore 的 `MajorCmd` “响应”不会被 deliver（等待超时且会走 unmatched）

## Code Review
- 结论：通过
- 重点关注：
  - 未放宽全局 `MajorCmd`：仅 VarStore 子协议进入兼容分支，避免普通 Cmd 帧被 await 误吞。

## 回滚方案
- 回滚 `await/client.go` 中 VarStore 白名单分支与对应单测即可恢复旧行为。
