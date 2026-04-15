package scanner

import (
	"net"

	"github.com/GoFurry/a2s-go"
	"github.com/GoFurry/a2s-go/master"
)

// Request describes one batch probe run.
type Request struct {
	Servers   []master.ServerAddr
	Discovery <-chan master.Result
}

// Result is one streamed probe result.
type Result struct {
	Server master.ServerAddr
	Info   *a2s.Info
	Err    error
}

// PlayersResult is one streamed A2S_PLAYER probe result.
type PlayersResult struct {
	Server  master.ServerAddr
	Players *a2s.Players
	Err     error
}

// RulesResult is one streamed A2S_RULES probe result.
type RulesResult struct {
	Server master.ServerAddr
	Rules  *a2s.Rules
	Err    error
}

func cloneServer(server master.ServerAddr) master.ServerAddr {
	return master.ServerAddr{
		IP:   cloneIPv4(server.IP),
		Port: server.Port,
	}
}

func cloneIPv4(ip net.IP) net.IP {
	if ip == nil {
		return nil
	}
	ipv4 := ip.To4()
	if ipv4 == nil {
		return nil
	}
	return append(net.IP(nil), ipv4...)
}
