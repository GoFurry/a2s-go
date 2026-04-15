# a2s-go

[English README](README.md)

![License](https://img.shields.io/badge/License-MIT-6C757D?style=flat&color=3B82F6)
![Release](https://img.shields.io/github/v/release/GoFurry/a2s-go?style=flat&color=blue)
![Go Version](https://img.shields.io/badge/Go-1.24%2B-00ADD8?style=flat&logo=go&logoColor=white)
[![Go Report Card](https://goreportcard.com/badge/github.com/GoFurry/a2s-go)](https://goreportcard.com/report/github.com/GoFurry/a2s-go)

`a2s-go` 是一个轻量、聚焦的 Go A2S UDP 查询库。

它把能力拆成三层：

- `a2s`：查询单个已知服务器的 `A2S_INFO` / `A2S_PLAYER` / `A2S_RULES`
- `master`：从 Valve master server 分页发现服务器地址
- `scanner`：把地址列表或 discovery 流转成批量探测结果

## 安装

```bash
go get github.com/GoFurry/a2s-go@latest
```

## 快速开始

### 查询单台服务器

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/GoFurry/a2s-go"
)

func main() {
	client, err := a2s.NewClient(
		"1.2.3.4:27015",
		a2s.WithTimeout(3*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	info, err := client.QueryInfo(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("name=%s map=%s players=%d/%d", info.Name, info.Map, info.Players, info.MaxPlayers)
}
```

### 从 Master 做地址发现

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/GoFurry/a2s-go/master"
)

func main() {
	client, err := master.NewClient(
		master.WithTimeout(5*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	req := master.Request{
		Region: master.RegionAsia,
		Filter: "\\secure\\1",
	}

	page, err := client.Query(context.Background(), req)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("servers=%d done=%v next=%s", len(page.Servers), page.Done, page.NextCursor.String())
}
```

`Query` 只拉一页，`Stream` 会自动翻页直到 discovery 结束。

### 批量探测服务器

```go
package main

import (
	"context"
	"log"

	"github.com/GoFurry/a2s-go/master"
	"github.com/GoFurry/a2s-go/scanner"
)

func main() {
	client, err := scanner.NewClient(
		scanner.WithConcurrency(32),
	)
	if err != nil {
		log.Fatal(err)
	}

	results, err := client.CollectInfo(context.Background(), scanner.Request{
		Servers: []master.ServerAddr{
			{IP: []byte{127, 0, 0, 1}, Port: 27015},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, result := range results {
		if result.Err != nil {
			log.Printf("probe error: %v", result.Err)
			continue
		}
		log.Printf("%s -> %s", result.Server.String(), result.Info.Name)
	}
}
```

`scanner` 还支持：

- `ProbePlayers` / `CollectPlayers`
- `ProbeRules` / `CollectRules`
- 直接消费 `master.Stream` 风格的 discovery 输入流

## 示例

- `go run ./examples/basic`
- `go run ./examples/master`
- `go run ./examples/master/fake-master`
- `go run ./examples/scanner`

## 参考资料

协议行为优先以 Valve 官方文档为准：

- [Valve Developer Wiki: Server queries](https://developer.valvesoftware.com/wiki/Server_queries#A2S_INFO)
- [Valve Developer Wiki: Master Server Query Protocol](https://developer.valvesoftware.com/wiki/Master_Server_Query_Protocol)