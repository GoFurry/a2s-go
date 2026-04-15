# a2s-go

`a2s-go` 是一个轻量、聚焦、长期可维护的 Go A2S 查询库。

它的目标不是做一个大而全的游戏服务器工具箱，而是先把 `A2S_INFO`、`A2S_PLAYER`、`A2S_RULES` 这三个最核心的查询能力，用清晰、稳定、可维护的方式做好。

## 项目定位

- 独立的 `A2S UDP Query` Go SDK
- 与 `steam-go` 主体分离，不混入 Steam Web API 能力
- 首版只覆盖：
  - `A2S_INFO`
  - `A2S_PLAYER`
  - `A2S_RULES`
- 首版只面向 `Source engine and above`

## 非目标

首版明确不做以下内容：

- 不支持 Goldsource
- 不做批量查询
- 不做缓存
- 不做监控或指标导出
- 不做服务发现
- 不做与 Steam Web API 的混合入口
- 不做面向具体游戏的专属 helper

## 设计目标

- 小而稳
- 类型安全
- 调用链简单
- `context` 驱动超时与取消
- 把 challenge、多包、压缩包等协议细节封装在内部

## 预期用法

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

## 首版能力

- 自动处理 challenge
- 自动处理多包响应
- 支持 bzip2 压缩包解压与校验
- 统一错误模型
- `context` 驱动超时与取消
- 单地址、单 client 复用 UDP 连接

## 项目结构

建议实现结构如下：

```text
a2s-go/
  README.md
  docs/
    architecture.md
    api-contract.md
  client.go
  options.go
  errors.go
  types.go
  internal/
    protocol/
    transport/
    challenge/
    multipacket/
    errors/
```

## 版本策略

- 建议支持 `Go 1.24+`
- 首版优先保证公开 API 稳定
- 内部实现细节不作为兼容承诺的一部分

## 参考资料

本项目以 Valve 官方协议说明为准：

- [Valve Developer Wiki: Server queries](https://developer.valvesoftware.com/wiki/Server_queries#A2S_INFO)

以下成熟库只作为参考输入，不作为 API 对齐目标：

- [rumblefrog/go-a2s](https://github.com/rumblefrog/go-a2s)

## 文档

- [架构设计](docs/architecture.md)
- [接口契约](docs/api-contract.md)
- [Master/Discovery 架构设计](docs/master-architecture.md)
- [Master/Discovery 接口契约](docs/master-api-contract.md)
- [Roadmap](roadmap.md)
