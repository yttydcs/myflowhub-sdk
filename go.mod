module github.com/yttydcs/myflowhub-sdk

go 1.23.0

toolchain go1.24.5

require (
	github.com/yttydcs/myflowhub-core v0.1.0
	github.com/yttydcs/myflowhub-proto v0.0.0
)

// 开发期本地联调：后续会通过为 core/proto 打 tag 并移除 replace 来发布稳定版本。
replace github.com/yttydcs/myflowhub-core => ../MyFlowHub-Core

replace github.com/yttydcs/myflowhub-proto => ../MyFlowHub-Proto

