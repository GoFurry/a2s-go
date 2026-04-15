package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/GoFurry/a2s-go"
)

func main() {
	addr := "45.125.45.95:28008"
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}

	client, err := a2s.NewClient(
		addr,
		a2s.WithTimeout(3*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, err := client.QueryInfo(ctx)
	if err != nil {
		log.Fatal(err)
	}

	players, err := client.QueryPlayers(ctx)
	if err != nil {
		log.Fatal(err)
	}

	rules, err := client.QueryRules(ctx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("== QueryInfo ==")
	fmt.Printf("name=%s map=%s players=%d/%d version=%s\n", info.Name, info.Map, info.Players, info.MaxPlayers, info.Version)

	fmt.Println()
	fmt.Println("== QueryPlayers ==")
	fmt.Printf("count=%d returned=%d\n", players.Count, len(players.Players))
	for i, player := range players.Players {
		if i >= 10 {
			fmt.Printf("... %d more players omitted\n", len(players.Players)-i)
			break
		}
		fmt.Printf("[%d] name=%s score=%d duration=%.2f\n", i+1, player.Name, player.Score, player.Duration)
	}

	fmt.Println()
	fmt.Println("== QueryRules ==")
	fmt.Printf("count=%d returned=%d\n", rules.Count, len(rules.Items))
	printed := 0
	for key, value := range rules.Items {
		fmt.Printf("%s=%s\n", key, value)
		printed++
		if printed >= 10 {
			if len(rules.Items) > printed {
				fmt.Printf("... %d more rules omitted\n", len(rules.Items)-printed)
			}
			break
		}
	}
}
