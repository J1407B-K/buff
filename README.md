## buff

`buff` 是一个支持标准库 `net/http` 与 [gnet](https://github.com/panjf2000/gnet) 双引擎的轻量级 HTTP 框架。

### 快速开始

```bash
# 标准 net/http 引擎
go run ./example -engine std -addr :8080

# gnet 引擎
go run ./example -engine gnet -addr :8080
```

默认路由：

- `GET /ping` -> `{"message":"pong"}`
- `POST /delay` -> 模拟延迟响应

### 压测基准

仓库提供 `BenchmarkEngineRun` 与 `BenchmarkEngineRunGNet`，可比较两种引擎在同一路径下的吞吐表现：

```bash
GOCACHE=$(pwd)/.gocache go test -run=^$ -bench '^BenchmarkEngineRun(GNet)?$' ./buff
```

在 Apple M4 本地环境测试，gnet 版本约比标准库快 20% 左右（依据 `/ping` 路由，实际效果视业务逻辑而定）。

### 开发

1. 安装依赖：`go mod tidy`
2. 运行单元测试：`GOCACHE=$(pwd)/.gocache go test ./...`
3. 运行示例服务并自定义中间件、路由。

欢迎按需扩展中间件、在压力测试中加入更复杂的业务场景。*** End Patch
