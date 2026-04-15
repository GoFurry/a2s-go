package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/GoFurry/a2s-go/master"
)

func main() {
	addr := flag.String("addr", "", "override master server address, for example 208.64.200.65:27011")
	flag.Parse()

	opts := []master.Option{
		master.WithTimeout(5 * time.Second),
	}
	if *addr != "" {
		opts = append(opts, master.WithBaseAddress(*addr))
	}

	client, err := master.NewClient(opts...)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := master.Request{
		Region: master.RegionAsia,
		Filter: "\\secure\\1",
	}

	page, err := client.Query(ctx, req)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("== Query ==")
	fmt.Printf("servers=%d done=%t next=%s\n", len(page.Servers), page.Done, page.NextCursor.String())
	for i, server := range page.Servers {
		if i >= 10 {
			fmt.Printf("... %d more servers omitted\n", len(page.Servers)-i)
			break
		}
		fmt.Println(server.String())
	}

	fmt.Println()
	fmt.Println("== Stream ==")
	stream, err := client.Stream(ctx, req)
	if err != nil {
		log.Fatal(err)
	}
	count := 0
	for result := range stream {
		if result.Err != nil {
			log.Fatal(result.Err)
		}
		fmt.Println(result.Server.String())
		count++
		if count >= 10 {
			fmt.Println("... stream output truncated after 10 servers")
			break
		}
	}
}
