package scanner

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/GoFurry/a2s-go"
	"github.com/GoFurry/a2s-go/master"
)

// Client probes batches of discovered servers.
type Client struct {
	concurrency   int
	timeout       time.Duration
	maxPacketSize int
}

// NewClient creates one scanner client.
func NewClient(opts ...Option) (*Client, error) {
	cfg := defaultClientConfig()
	for _, opt := range opts {
		if opt == nil {
			return nil, newError(ErrorCodeInput, "new_client", zeroServer, "option must not be nil", nil)
		}
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	return &Client{
		concurrency:   cfg.concurrency,
		timeout:       cfg.timeout,
		maxPacketSize: cfg.maxPacketSize,
	}, nil
}

// Probe runs one batch of A2S_INFO probes and streams the results as they finish.
func (c *Client) Probe(ctx context.Context, req Request) (<-chan Result, error) {
	return runProbe(
		c,
		ctx,
		req,
		c.probeInfoOne,
		func(server master.ServerAddr, err error) Result {
			return Result{
				Server: cloneServer(server),
				Err:    newError(ErrorCodeDiscovery, "probe", server, "discovery input failed", err),
			}
		},
	)
}

// Collect runs Probe and returns all results in completion order.
func (c *Client) Collect(ctx context.Context, req Request) ([]Result, error) {
	return c.CollectInfo(ctx, req)
}

// ProbeInfo runs one batch of A2S_INFO probes and streams the results as they finish.
func (c *Client) ProbeInfo(ctx context.Context, req Request) (<-chan Result, error) {
	return c.Probe(ctx, req)
}

// CollectInfo runs ProbeInfo and returns all results in completion order.
func (c *Client) CollectInfo(ctx context.Context, req Request) ([]Result, error) {
	stream, err := c.Probe(ctx, req)
	if err != nil {
		return nil, err
	}
	return collect(stream), nil
}

// ProbePlayers runs one batch of A2S_PLAYER probes and streams the results as they finish.
func (c *Client) ProbePlayers(ctx context.Context, req Request) (<-chan PlayersResult, error) {
	return runProbe(
		c,
		ctx,
		req,
		c.probePlayersOne,
		func(server master.ServerAddr, err error) PlayersResult {
			return PlayersResult{
				Server: cloneServer(server),
				Err:    newError(ErrorCodeDiscovery, "probe_players", server, "discovery input failed", err),
			}
		},
	)
}

// CollectPlayers runs ProbePlayers and returns all results in completion order.
func (c *Client) CollectPlayers(ctx context.Context, req Request) ([]PlayersResult, error) {
	stream, err := c.ProbePlayers(ctx, req)
	if err != nil {
		return nil, err
	}
	return collect(stream), nil
}

// ProbeRules runs one batch of A2S_RULES probes and streams the results as they finish.
func (c *Client) ProbeRules(ctx context.Context, req Request) (<-chan RulesResult, error) {
	return runProbe(
		c,
		ctx,
		req,
		c.probeRulesOne,
		func(server master.ServerAddr, err error) RulesResult {
			return RulesResult{
				Server: cloneServer(server),
				Err:    newError(ErrorCodeDiscovery, "probe_rules", server, "discovery input failed", err),
			}
		},
	)
}

// CollectRules runs ProbeRules and returns all results in completion order.
func (c *Client) CollectRules(ctx context.Context, req Request) ([]RulesResult, error) {
	stream, err := c.ProbeRules(ctx, req)
	if err != nil {
		return nil, err
	}
	return collect(stream), nil
}

func validateRequest(req Request) error {
	hasServers := req.Servers != nil
	hasDiscovery := req.Discovery != nil
	if hasServers == hasDiscovery {
		return newError(ErrorCodeInput, "probe", zeroServer, "request must set exactly one input source", nil)
	}
	return nil
}

func runProbe[T any](
	c *Client,
	ctx context.Context,
	req Request,
	probeOne func(context.Context, master.ServerAddr) T,
	discoveryErr func(master.ServerAddr, error) T,
) (<-chan T, error) {
	if c == nil {
		return nil, newError(ErrorCodeInput, "probe", zeroServer, "client is nil", nil)
	}
	if err := validateRequest(req); err != nil {
		return nil, err
	}

	results := make(chan T)
	jobs := make(chan master.ServerAddr)

	var workers sync.WaitGroup
	for i := 0; i < c.concurrency; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for server := range jobs {
				results <- probeOne(ctx, server)
			}
		}()
	}

	go func() {
		defer close(jobs)
		if req.Servers != nil {
			c.feedServers(ctx, jobs, req.Servers)
			return
		}
		feedDiscovery(ctx, jobs, results, req.Discovery, discoveryErr)
	}()

	go func() {
		workers.Wait()
		close(results)
	}()

	return results, nil
}

func (c *Client) feedServers(ctx context.Context, jobs chan<- master.ServerAddr, servers []master.ServerAddr) {
	for _, server := range servers {
		select {
		case <-ctx.Done():
			return
		case jobs <- cloneServer(server):
		}
	}
}

func feedDiscovery[T any](
	ctx context.Context,
	jobs chan<- master.ServerAddr,
	results chan<- T,
	discovery <-chan master.Result,
	discoveryErr func(master.ServerAddr, error) T,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case discovered, ok := <-discovery:
			if !ok {
				return
			}
			if discovered.Err != nil {
				results <- discoveryErr(discovered.Server, discovered.Err)
				continue
			}
			select {
			case <-ctx.Done():
				return
			case jobs <- cloneServer(discovered.Server):
			}
		}
	}
}

func (c *Client) newA2SClient(server master.ServerAddr) (*a2s.Client, error) {
	return a2s.NewClient(
		server.String(),
		a2s.WithTimeout(c.timeout),
		a2s.WithMaxPacketSize(c.maxPacketSize),
	)
}

func (c *Client) probeInfoOne(ctx context.Context, server master.ServerAddr) Result {
	client, err := c.newA2SClient(server)
	if err != nil {
		return Result{
			Server: cloneServer(server),
			Err:    newError(ErrorCodeProbe, "probe", server, "create a2s client failed", err),
		}
	}
	defer client.Close()

	info, err := client.QueryInfo(ctx)
	if err != nil {
		return Result{
			Server: cloneServer(server),
			Err:    mapProbeError("probe", server, err),
		}
	}
	return Result{
		Server: cloneServer(server),
		Info:   info,
	}
}

func (c *Client) probePlayersOne(ctx context.Context, server master.ServerAddr) PlayersResult {
	client, err := c.newA2SClient(server)
	if err != nil {
		return PlayersResult{
			Server: cloneServer(server),
			Err:    newError(ErrorCodeProbe, "probe_players", server, "create a2s client failed", err),
		}
	}
	defer client.Close()

	players, err := client.QueryPlayers(ctx)
	if err != nil {
		return PlayersResult{
			Server:  cloneServer(server),
			Err:     mapProbeError("probe_players", server, err),
			Players: nil,
		}
	}
	return PlayersResult{
		Server:  cloneServer(server),
		Players: players,
	}
}

func (c *Client) probeRulesOne(ctx context.Context, server master.ServerAddr) RulesResult {
	client, err := c.newA2SClient(server)
	if err != nil {
		return RulesResult{
			Server: cloneServer(server),
			Err:    newError(ErrorCodeProbe, "probe_rules", server, "create a2s client failed", err),
		}
	}
	defer client.Close()

	rules, err := client.QueryRules(ctx)
	if err != nil {
		return RulesResult{
			Server: cloneServer(server),
			Err:    mapProbeError("probe_rules", server, err),
			Rules:  nil,
		}
	}
	return RulesResult{
		Server: cloneServer(server),
		Rules:  rules,
	}
}

func mapProbeError(op string, server master.ServerAddr, err error) error {
	if err == nil {
		return nil
	}
	var a2sErr *a2s.Error
	if errors.As(err, &a2sErr) {
		code := ErrorCodeProbe
		if a2sErr.Code == a2s.ErrorCodeTimeout {
			code = ErrorCodeTimeout
		}
		return newError(code, op, server, a2sErr.Message, err)
	}
	return newError(ErrorCodeProbe, op, server, fmt.Sprintf("probe failed: %v", err), err)
}

func collect[T any](stream <-chan T) []T {
	var results []T
	for result := range stream {
		results = append(results, result)
	}
	return results
}
