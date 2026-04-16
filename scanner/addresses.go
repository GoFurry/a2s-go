package scanner

import (
	"net"
	"strconv"
	"strings"

	"github.com/GoFurry/a2s-go/master"
)

const defaultServerPort = 27015

func parseAddresses(addrs []string) ([]master.ServerAddr, error) {
	servers := make([]master.ServerAddr, 0, len(addrs))
	for _, addr := range addrs {
		server, err := parseAddress(addr)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}
	return servers, nil
}

func parseAddress(addr string) (master.ServerAddr, error) {
	trimmed := strings.TrimSpace(addr)
	if trimmed == "" {
		return zeroServer, newError(ErrorCodeInput, "parse_address", zeroServer, "address must not be empty", nil)
	}

	host, port, err := net.SplitHostPort(trimmed)
	if err != nil {
		if strings.Contains(err.Error(), "missing port in address") {
			trimmed = net.JoinHostPort(trimmed, strconv.Itoa(defaultServerPort))
		} else {
			return zeroServer, newError(ErrorCodeInput, "parse_address", zeroServer, "invalid address", err)
		}
	} else {
		if host == "" {
			return zeroServer, newError(ErrorCodeInput, "parse_address", zeroServer, "host must not be empty", nil)
		}
		if port == "" {
			trimmed = net.JoinHostPort(host, strconv.Itoa(defaultServerPort))
		}
	}

	udpAddr, err := net.ResolveUDPAddr("udp4", trimmed)
	if err != nil {
		return zeroServer, newError(ErrorCodeInput, "parse_address", zeroServer, "resolve udp address failed", err)
	}
	if udpAddr.IP == nil || udpAddr.IP.To4() == nil {
		return zeroServer, newError(ErrorCodeInput, "parse_address", zeroServer, "scanner only supports IPv4 addresses", nil)
	}

	return master.ServerAddr{
		IP:   append(net.IP(nil), udpAddr.IP.To4()...),
		Port: uint16(udpAddr.Port),
	}, nil
}
