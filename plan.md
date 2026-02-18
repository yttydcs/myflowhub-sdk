# Plan - SDK：发布 v0.1.0 + 移除 replace（semver 依赖化）（PR19-SDK-SemVer）

## Workflow 信息
- Repo：`MyFlowHub-SDK`
- 分支：`chore/sdk-semver`
- Worktree：`d:\project\MyFlowHub3\worktrees\pr19-semver-deps\MyFlowHub-SDK`
- Base：`main`
- 参考：
  - `d:\project\MyFlowHub3\target.md`
  - `d:\project\MyFlowHub3\repos.md`
  - `d:\project\MyFlowHub3\guide.md`（commit 信息中文）
- 目标：
  - 发布 `github.com/yttydcs/myflowhub-sdk@v0.1.0`
  - SDK 自身移除 `replace`，并改为依赖：
    - `github.com/yttydcs/myflowhub-core@v0.2.0`
    - `github.com/yttydcs/myflowhub-proto@v0.1.0`

## 约束（边界）
- 本 workflow 不新增业务能力；仅做“依赖可拉取化”与版本发布：
  - 允许：`go.mod/go.sum` 调整、文档归档（`docs/change`）。
  - 禁止：破坏性 API 变更（若必须，需回到 3.1 更新计划并重新确认）。
- 验收必须使用 `GOWORK=off`（确保单仓 clone 可用）。

## 当前状态（事实，可审计）
- SDK 当前通过 `replace ../MyFlowHub-Core` / `replace ../MyFlowHub-Proto` 联调。
- 上游 Win 已依赖 SDK；本 workflow 要保证 Win 在无 replace 条件下可拉取 SDK 及其依赖。

---

## 1) 需求分析

### 目标
1) SDK 发布 `v0.1.0`（可被 `go get` 拉取）。
2) SDK `go.mod` 移除 `replace`，并锁定依赖到 `core v0.2.0 + proto v0.1.0`。
3) 回归通过：`GOWORK=off go test ./...`。

### 范围（必须 / 不做）
#### 必须
- 修改：
  - `go.mod`（移除 replace；更新 require 版本）
  - `go.sum`（随 `go mod tidy` 更新）
- 新增归档文档：`docs/change/2026-02-18_sdk-v0.1.0.md`
- 回归验证：`GOWORK=off go test ./... -count=1 -p 1`
- 结束 workflow 且你确认后：
  - 在 `repo/` 合并分支到 `main` 并 push
  - 创建 annotated tag `v0.1.0` 并 push tag

#### 不做
- 不引入新的模块依赖层级（仍只依赖 Core + Proto + 标准库）

### 验收标准
- `GOWORK=off go test ./... -count=1 -p 1` 通过
- 远端存在并可拉取：`github.com/yttydcs/myflowhub-sdk@v0.1.0`
- `go list -m` 可看到：
  - `github.com/yttydcs/myflowhub-core v0.2.0`
  - `github.com/yttydcs/myflowhub-proto v0.1.0`

### 风险
- 如果 Core/Proto tag 未发布或版本号不一致，将导致 `go mod tidy`/拉取失败；需严格按依赖顺序执行。

---

## 2) 架构设计（分析）

### 总体方案（采用）
- SDK 的依赖从本地 replace 迁移到 semver：
  - `core v0.2.0`
  - `proto v0.1.0`
- 本地多仓联调通过 `d:\project\MyFlowHub3\go.work`（不提交）解决；发布验收通过 `GOWORK=off` 保证可复现。

### 测试策略
- `GOWORK=off go test ./...`（确保依赖真实拉取）

---

## 3.1) 计划拆分（Checklist）

## 问题清单（阻塞：否）
- 已确认版本：SDK 发布 `v0.1.0`；依赖 core=`v0.2.0`、proto=`v0.1.0`；验收使用 `GOWORK=off`。

### SDKSEM1 - 调整 go.mod/go.sum（移除 replace + 固定版本）
- 目标：让 SDK 可在无 go.work/无 replace 的情况下构建。
- 涉及文件：
  - `go.mod`
  - `go.sum`
- 验收条件：
  - `replace` 均移除；
  - `require` 指向 `core v0.2.0`、`proto v0.1.0`；
  - `go mod tidy` 后工作区干净。
- 测试点：
  - `GOWORK=off go test ./...`
- 回滚点：
  - revert 该提交。

### SDKSEM2 - 回归测试（GOWORK=off）
- 目标：确保依赖真实拉取且测试通过。
- 命令：
  - `$env:GOTMPDIR='d:\\project\\MyFlowHub3\\.tmp\\gotmp'`
  - `New-Item -ItemType Directory -Force -Path $env:GOTMPDIR | Out-Null`
  - `$env:GOWORK='off'`
  - `go test ./... -count=1 -p 1`
- 验收条件：通过。
- 回滚点：revert 本分支改动。

### SDKSEM3 - 归档发布文档
- 目标：记录 SDK `v0.1.0` 的发布内容、依赖版本、验证方式与回滚策略。
- 涉及文件：
  - `docs/change/2026-02-18_sdk-v0.1.0.md`
- 验收条件：文档可用于他人独立复现验证。
- 回滚点：revert 文档提交。

### SDKSEM4 - Code Review（阶段 3.3）+ 归档（阶段 4）
- 目标：Review 覆盖需求/风险/测试；归档记录完整。
- 验收条件：Review 结论为“通过”。

### SDKSEM5 - 合并与打 tag（你确认结束 workflow 后执行）
- 目标：合并到 `main` 并发布 `v0.1.0`。
- 步骤（在 `repo/MyFlowHub-SDK` 执行）：
  1) `git merge --ff-only origin/chore/sdk-semver`
  2) `git push origin main`
  3) `git tag -a v0.1.0 -m \"chore: 发布 v0.1.0\"`
  4) `git push origin v0.1.0`
- 验收条件：tag 可被 `go get` 拉取。
- 回滚方案：不删除 tag；如需修复，追加发布 `v0.1.1`。
