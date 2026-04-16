package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/GoFurry/a2s-go"
	"github.com/GoFurry/a2s-go/master"
	"github.com/GoFurry/a2s-go/scanner"
)

type singleResult struct {
	server  master.ServerAddr
	info    *a2s.Info
	players *a2s.Players
	rules   *a2s.Rules
	errs    []error
}

func main() {
	var (
		serversCSV  = flag.String("servers", "", "comma-separated host[:port] targets for manual live regression")
		timeout     = flag.Duration("timeout", 3*time.Second, "per-request timeout")
		concurrency = flag.Int("concurrency", 16, "scanner worker concurrency")
		mode        = flag.String("mode", "all", "one of: all, info, players, rules")
		runScanner  = flag.Bool("scanner", true, "run scanner batch probes after single-server probes")
		playerLimit = flag.Int("players-limit", 5, "max players to print per server")
		rulesLimit  = flag.Int("rules-limit", 10, "max rules to print per server")
	)
	flag.Parse()

	if strings.TrimSpace(*serversCSV) == "" {
		log.Fatal("missing -servers, for example: -servers=1.2.3.4:27015,5.6.7.8")
	}

	serverInputs := splitCSV(*serversCSV)
	servers, err := scanner.ParseAddresses(serverInputs)
	if err != nil {
		log.Fatal(err)
	}

	if err := validateMode(*mode); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	fmt.Println("== Single-server probes ==")
	singleResults := make([]singleResult, 0, len(servers))
	for _, server := range servers {
		result := runSingle(ctx, server, *timeout, *mode)
		singleResults = append(singleResults, result)
		printSingle(result, *playerLimit, *rulesLimit)
	}

	if *runScanner {
		fmt.Println()
		fmt.Println("== Scanner probes ==")
		if err := runScannerPass(ctx, servers, *timeout, *concurrency, *mode); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Println()
	fmt.Println("== Summary ==")
	var successCount int
	for _, result := range singleResults {
		if len(result.errs) == 0 {
			successCount++
		}
	}
	fmt.Printf("single success=%d/%d\n", successCount, len(singleResults))
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func validateMode(mode string) error {
	switch mode {
	case "all", "info", "players", "rules":
		return nil
	default:
		return fmt.Errorf("invalid -mode %q", mode)
	}
}

func runSingle(ctx context.Context, server master.ServerAddr, timeout time.Duration, mode string) singleResult {
	result := singleResult{server: server}

	client, err := a2s.NewClient(
		server.String(),
		a2s.WithTimeout(timeout),
	)
	if err != nil {
		result.errs = append(result.errs, err)
		return result
	}
	defer client.Close()

	probeCtx, cancel := context.WithTimeout(ctx, timeout*2)
	defer cancel()

	if mode == "all" || mode == "info" {
		info, err := client.QueryInfo(probeCtx)
		if err != nil {
			result.errs = append(result.errs, fmt.Errorf("info: %w", err))
		} else {
			result.info = info
		}
	}
	if mode == "all" || mode == "players" {
		players, err := client.QueryPlayers(probeCtx)
		if err != nil {
			result.errs = append(result.errs, fmt.Errorf("players: %w", err))
		} else {
			result.players = players
		}
	}
	if mode == "all" || mode == "rules" {
		rules, err := client.QueryRules(probeCtx)
		if err != nil {
			result.errs = append(result.errs, fmt.Errorf("rules: %w", err))
		} else {
			result.rules = rules
		}
	}

	return result
}

func printSingle(result singleResult, playerLimit int, rulesLimit int) {
	fmt.Printf("-- %s --\n", result.server.String())
	if result.info != nil {
		fmt.Printf("info: name=%s map=%s players=%d/%d version=%s\n",
			result.info.Name, result.info.Map, result.info.Players, result.info.MaxPlayers, result.info.Version)
	}
	if result.players != nil {
		fmt.Printf("players: count=%d returned=%d\n", result.players.Count, len(result.players.Players))
		for i, player := range result.players.Players {
			if i >= playerLimit {
				fmt.Printf("players: ... %d more omitted\n", len(result.players.Players)-i)
				break
			}
			fmt.Printf("players: [%d] %s score=%d duration=%.2f\n", i+1, player.Name, player.Score, player.Duration)
		}
	}
	if result.rules != nil {
		fmt.Printf("rules: count=%d returned=%d truncated=%t\n", result.rules.ReportedCount, len(result.rules.Items), result.rules.Truncated)
		keys := make([]string, 0, len(result.rules.Items))
		for key := range result.rules.Items {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for i, key := range keys {
			if i >= rulesLimit {
				fmt.Printf("rules: ... %d more omitted\n", len(keys)-i)
				break
			}
			fmt.Printf("rules: %s=%s\n", key, result.rules.Items[key])
		}
	}
	for _, err := range result.errs {
		fmt.Printf("error: %v\n", err)
	}
}

func runScannerPass(ctx context.Context, servers []master.ServerAddr, timeout time.Duration, concurrency int, mode string) error {
	client, err := scanner.NewClient(
		scanner.WithTimeout(timeout),
		scanner.WithConcurrency(concurrency),
	)
	if err != nil {
		return err
	}

	req := scanner.Request{Servers: servers}
	switch mode {
	case "all", "info":
		results, err := client.CollectInfo(ctx, req)
		if err != nil {
			return err
		}
		printScannerInfo(results)
		if mode != "all" {
			return nil
		}
		fallthrough
	case "players":
		results, err := client.CollectPlayers(ctx, req)
		if err != nil {
			return err
		}
		printScannerPlayers(results)
		if mode != "all" {
			return nil
		}
		fallthrough
	case "rules":
		results, err := client.CollectRules(ctx, req)
		if err != nil {
			return err
		}
		printScannerRules(results)
	}
	return nil
}

func printScannerInfo(results []scanner.Result) {
	fmt.Println("scanner info:")
	for _, result := range results {
		if result.Err != nil {
			fmt.Printf("  %s -> error: %v\n", result.Server.String(), result.Err)
			continue
		}
		fmt.Printf("  %s -> %s\n", result.Server.String(), result.Info.Name)
	}
}

func printScannerPlayers(results []scanner.PlayersResult) {
	fmt.Println("scanner players:")
	for _, result := range results {
		if result.Err != nil {
			fmt.Printf("  %s -> error: %v\n", result.Server.String(), result.Err)
			continue
		}
		fmt.Printf("  %s -> count=%d\n", result.Server.String(), result.Players.Count)
	}
}

func printScannerRules(results []scanner.RulesResult) {
	fmt.Println("scanner rules:")
	for _, result := range results {
		if result.Err != nil {
			fmt.Printf("  %s -> error: %v\n", result.Server.String(), result.Err)
			continue
		}
		fmt.Printf("  %s -> rules=%d truncated=%t\n", result.Server.String(), len(result.Rules.Items), result.Rules.Truncated)
	}
}
