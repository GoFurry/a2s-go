# a2s-go

`a2s-go` 是一个轻量、聚焦、长期可维护的 Go A2S 查询库。

它不打算做成一个大而全的游戏服务器工具箱，而是把 A2S 相关能力拆成清晰的两层：

- 根包 `a2s`：查询单台已知服务器的 `A2S_INFO`、`A2S_PLAYER`、`A2S_RULES`
- 子包 `master`：从 Valve Master Server 分页发现服务器地址

## 项目定位

- 独立的 `A2S UDP Query` Go SDK
- 与 `steam-go` 主体分离，不混入 Steam Web API 能力
- 当前公开能力：`a2s.QueryInfo`、`a2s.QueryPlayers`、`a2s.QueryRules`、`master.Query`、`master.Stream`
- 当前协议边界：单服查询面向 `Source engine and above`；discovery 首版只承诺 IPv4

## 非目标

当前版本明确不做以下内容：

- 不支持 Goldsource
- 不做高并发扫描器
- 不做 worker pool
- 不做缓存
- 不做监控或指标导出
- 不做与 Steam Web API 的混合入口
- 不做面向具体游戏的专属 helper

## 设计目标

- 小而稳
- 类型安全
- 调用链简单
- `context` 驱动超时与取消
- 把 challenge、多包、压缩包、master 分页协议等细节封装在内部

## 单服查询

```go
package main

import (
	"context"
	"time"

	"github.com/GoFurry/a2s-go"
)

func main() {
	client, err := a2s.NewClient(
		"1.2.3.4:27015",
		a2s.WithTimeout(3*time.Second),
	)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	info, err := client.QueryInfo(context.Background())
	if err != nil {
		panic(err)
	}

	players, err := client.QueryPlayers(context.Background())
	if err != nil {
		panic(err)
	}

	rules, err := client.QueryRules(context.Background())
	if err != nil {
		panic(err)
	}

	_, _, _ = info, players, rules
}
```

## Master Discovery

```go
package main

import (
	"context"
	"time"

	"github.com/GoFurry/a2s-go/master"
)

func main() {
	client, err := master.NewClient(
		master.WithTimeout(5*time.Second),
	)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	req := master.Request{
		Region: master.RegionAsia,
		Filter: "\\secure\\1",
	}

	page, err := client.Query(context.Background(), req)
	if err != nil {
		panic(err)
	}

	stream, err := client.Stream(context.Background(), req)
	if err != nil {
		panic(err)
	}

	_, _, _ = page, stream, time.Second
}
```

`Query` 只拉一页；`Stream` 会自动推进 cursor，逐个输出发现到的地址。

## 当前能力

- `a2s` 根包：自动处理 challenge、split packet、bzip2 压缩包解压与校验，提供统一错误模型，并复用单地址 UDP 连接
- `master` 子包：提供 `region / filter / cursor` 稳定契约、单页/流式 discovery 查询、统一错误模型，以及单 master server UDP 连接复用

## 项目结构

```text
a2s-go/
  README.md
  docs/
    architecture.md
    api-contract.md
    master-architecture.md
    master-api-contract.md
  client.go
  options.go
  errors.go
  types.go
  master/
    client.go
    options.go
    errors.go
    types.go
  internal/
    protocol/
    transport/
    challenge/
    multipacket/
    masterprotocol/
    errors/
```

## 版本策略

- 建议支持 `Go 1.24+`
- 优先保证公开 API 稳定
- 内部实现细节不作为兼容承诺的一部分

## 参考资料

项目实现以 Valve 官方协议说明为准：

- [Valve Developer Wiki: Server queries](https://developer.valvesoftware.com/wiki/Server_queries#A2S_INFO)
- [Valve Developer Wiki: Master Server Query Protocol](https://developer.valvesoftware.com/wiki/Master_Server_Query_Protocol)

以下成熟库只作为参考输入，不作为 API 对齐目标：

- [rumblefrog/go-a2s](https://github.com/rumblefrog/go-a2s)

## 文档

- [架构设计](docs/architecture.md)
- [接口契约](docs/api-contract.md)
- [Master/Discovery 架构设计](docs/master-architecture.md)
- [Master/Discovery 接口契约](docs/master-api-contract.md)
- [Roadmap](roadmap.md)
