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

### gnet 用法说明

`buff.Engine` 同时兼容标准库与 gnet 两套运行时；若想直接以 gnet 驱动，可调用 `RunGNet` 并按需传入配置项：

```go
eng := buff.New()

eng.GET("/ping", func(c *buff.Context) {
	c.JSON(http.StatusOK, map[string]string{"message": "pong"})
})

if err := eng.RunGNet(":8080",
	buff.WithGNetMaxHeaderBytes(16<<10),
	buff.WithGNetServerHeader("buff-gnet-demo"),
); err != nil {
	log.Fatal(err)
}
```

HTTP 报文的解析完全由 `buff` 托管，当前实现已经覆盖：

- `Content-Length` 与 `Transfer-Encoding: chunked`（含 trailer）；
- HTTP/1.0 `Connection: keep-alive` 及显式 `Connection: close` 处理；
- 自定义 `Server` 响应头、最大 Header 限制、优雅停机信号。

如果你不需要 gnet 带来的高并发优势，依旧可以调用 `Run`/`RunWithServer` 保持标准库行为，两套 API 共享路由与中间件。

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

欢迎按需扩展中间件、在压力测试中加入更复杂的业务场景。
