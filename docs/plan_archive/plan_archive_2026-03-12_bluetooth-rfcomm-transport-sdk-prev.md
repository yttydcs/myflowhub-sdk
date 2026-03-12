# Plan - MyFlowHub-SDK：await 兼容 VarStore `MajorCmd` 响应

## Workflow 信息
- Repo：`MyFlowHub-SDK`
- 分支：`refactor/varstore-hop-align`
- Worktree：`d:\project\MyFlowHub3\worktrees\varstore-hop-align\sdk`
- Base：`main`
- 关联仓库：`MyFlowHub-SubProto`、`MyFlowHub-Win`、`MyFlowHub-Server`

## 项目目标与当前状态
- 目标：在不破坏现有 await 行为的前提下，支持 VarStore 响应走 `MajorCmd`。
- 当前状态：已完成 SDK-1/SDK-2：await 对 VarStore 子协议新增 `MajorCmd` 白名单兼容路径，并补齐单测验证；`go test ./...` 通过。

## 依赖关系
- 依赖 SubProto 新行为定义（resp 逐跳可见）。
- Win 侧服务调用依赖 SDK await 行为，需联调验证。

## 风险与注意事项
- 不能全局放宽到所有 `MajorCmd`，避免误匹配普通命令帧。
- 匹配条件需保留 `subproto + action + msg_id/trace_id` 的约束。

## 可执行任务清单（Checklist）

### SDK-1 await 匹配策略扩展
- 目标：为 VarStore 响应引入 `MajorCmd` 兼容路径（白名单 action 或按 await 选项控制）。
- 涉及模块/文件：`await/client.go`（及相关 matcher/option 文件）
- 验收条件：VarStore `*_resp` 为 `MajorCmd` 时可正常命中 await。
- 测试点：VarStore 响应 `MajorCmd` 命中；非目标 Cmd 帧不误命中。
- 回滚点：恢复旧 major 匹配策略。

### SDK-2 回归兼容与测试
- 目标：保证现有 `MajorOKResp/MajorErrResp` 流程不回归。
- 涉及模块/文件：`await/*_test.go`
- 验收条件：旧用例继续通过；新增 VarStore Cmd 响应用例通过。
- 测试点：超时、取消、多并发 await 匹配。
- 回滚点：回退新增 matcher 分支。

### SDK-3 归档变更
- 目标：沉淀本仓改动、风险与回滚策略。
- 涉及模块/文件：`docs/change/2026-03-06_varstore-cmd-await-sdk.md`
- 验收条件：文档映射 SDK-1~SDK-2，给出联调命令。
- 测试点：文档中的命令可复现验证。
- 回滚点：文档回退不影响功能代码。
