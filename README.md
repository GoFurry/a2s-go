# a2s-go

[中文文档](README_zh.md)

![License](https://img.shields.io/badge/License-MIT-6C757D?style=flat&color=3B82F6)
![Release](https://img.shields.io/github/v/release/GoFurry/a2s-go?style=flat&color=blue)
![Go Version](https://img.shields.io/badge/Go-1.24%2B-00ADD8?style=flat&logo=go&logoColor=white)
[![Go Report Card](https://goreportcard.com/badge/github.com/GoFurry/a2s-go)](https://goreportcard.com/report/github.com/GoFurry/a2s-go)

`a2s-go` is a focused Go SDK for Steam/Source A2S UDP queries.

It is intentionally split into three layers:

- `a2s`: query a known game server with `A2S_INFO`, `A2S_PLAYER`, and `A2S_RULES`
- `master`: discover server addresses from Valve master server pagination
- `scanner`: turn address lists or discovery streams into batched probe results

## Install

```bash
go get github.com/GoFurry/a2s-go@latest
```

## Quick Start

### Query One Server

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

### Discover Servers From Master

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

`Query` fetches one page. `Stream` keeps turning pages until discovery is done.

### Batch Probe Servers

```go
package main

import (
	"context"
	"log"

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
		Addresses: []string{"127.0.0.1:27015"},
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

The scanner also supports:

- direct `[]string` address input via `scanner.Request{Addresses: ...}`
- `ProbePlayers` / `CollectPlayers`
- `ProbeRules` / `CollectRules`
- `master.Stream` style discovery input

## Examples

- `go run ./examples/basic`
- `go run ./examples/master`
- `go run ./examples/master/fake-master`
- `go run ./examples/scanner`

## References

Protocol behavior follows Valve documentation first:

- [Valve Developer Wiki: Server queries](https://developer.valvesoftware.com/wiki/Server_queries#A2S_INFO)
- [Valve Developer Wiki: Master Server Query Protocol](https://developer.valvesoftware.com/wiki/Master_Server_Query_Protocol)
