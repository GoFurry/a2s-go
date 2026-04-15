package a2s

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/GoFurry/a2s-go/internal/challenge"
	ierrors "github.com/GoFurry/a2s-go/internal/errors"
	"github.com/GoFurry/a2s-go/internal/multipacket"
	"github.com/GoFurry/a2s-go/internal/protocol"
	"github.com/GoFurry/a2s-go/internal/transport"
)

// Client queries one target server via UDP.
type Client struct {
	addr          string
	timeout       time.Duration
	maxPacketSize int

	conn   *net.UDPConn
	closed bool
	mu     sync.Mutex
}

// NewClient creates a new A2S client for one target address.
func NewClient(addr string, opts ...Option) (*Client, error) {
	cfg := defaultClientConfig()
	for _, opt := range opts {
		if opt == nil {
			return nil, newError(ErrorCodeAddress, "new_client", addr, "option must not be nil", nil)
		}
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	normalized, err := normalizeAddress(addr)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, normalized)
	if err != nil {
		return nil, newError(ErrorCodeDial, "new_client", normalized.String(), "dial udp failed", err)
	}

	return &Client{
		addr:          normalized.String(),
		timeout:       cfg.timeout,
		maxPacketSize: cfg.maxPacketSize,
		conn:          conn,
	}, nil
}

// Close releases the underlying UDP connection.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true
	if c.conn == nil {
		return nil
	}
	if err := c.conn.Close(); err != nil {
		return newError(ErrorCodeDial, "close", c.addr, "close udp connection failed", err)
	}
	return nil
}

// QueryInfo sends an A2S_INFO query.
func (c *Client) QueryInfo(ctx context.Context) (*Info, error) {
	if err := c.ensureUsable(); err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	resp, err := c.doQuery(ctx, "query_info", protocol.BuildInfoRequest(), protocol.HeaderInfo)
	if err != nil {
		return nil, err
	}
	info, err := parseInfo(resp)
	if err != nil {
		return nil, mapInternalError(err, "decode_info", c.addr)
	}
	return info, nil
}

// QueryPlayers sends an A2S_PLAYER query.
func (c *Client) QueryPlayers(ctx context.Context) (*Players, error) {
	if err := c.ensureUsable(); err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	resp, err := c.doQuery(ctx, "query_players", protocol.BuildPlayersRequest(challenge.NoChallenge), protocol.HeaderPlayers)
	if err != nil {
		return nil, err
	}
	players, err := parsePlayers(resp)
	if err != nil {
		return nil, mapInternalError(err, "decode_players", c.addr)
	}
	return players, nil
}

// QueryRules sends an A2S_RULES query.
func (c *Client) QueryRules(ctx context.Context) (*Rules, error) {
	if err := c.ensureUsable(); err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	resp, err := c.doQuery(ctx, "query_rules", protocol.BuildRulesRequest(challenge.NoChallenge), protocol.HeaderRules)
	if err != nil {
		return nil, err
	}
	rules, err := parseRules(resp)
	if err != nil {
		return nil, mapInternalError(err, "decode_rules", c.addr)
	}
	return rules, nil
}

func (c *Client) ensureUsable() error {
	if c == nil {
		return newError(ErrorCodeDial, "ensure_usable", "", "client is nil", nil)
	}
	if c.closed {
		return newError(ErrorCodeDial, "ensure_usable", c.addr, "client is closed", nil)
	}
	return nil
}

func (c *Client) doQuery(ctx context.Context, op string, request []byte, expectedHeader byte) ([]byte, error) {
	if err := transport.Send(ctx, c.conn, request, c.deadline(ctx)); err != nil {
		return nil, mapTransportError(err, op, c.addr, true)
	}

	packet, err := transport.Receive(ctx, c.conn, c.maxPacketSize, c.deadline(ctx))
	if err != nil {
		return nil, mapTransportError(err, op, c.addr, false)
	}

	packet, err = c.resolvePacket(ctx, op, request, packet, expectedHeader)
	if err != nil {
		return nil, err
	}
	return packet, nil
}

func (c *Client) resolvePacket(ctx context.Context, op string, request []byte, packet []byte, expectedHeader byte) ([]byte, error) {
	for {
		kind, err := protocol.ClassifyPacket(packet)
		if err != nil {
			return nil, mapInternalError(err, op, c.addr)
		}

		switch kind {
		case protocol.PacketSingle:
			header, err := protocol.PeekPayloadHeader(packet)
			if err != nil {
				return nil, mapInternalError(err, op, c.addr)
			}
			if header == protocol.HeaderChallenge {
				token, err := challenge.Parse(packet)
				if err != nil {
					return nil, mapInternalError(err, op, c.addr)
				}
				request = protocol.ApplyChallenge(request, token)
				if err := transport.Send(ctx, c.conn, request, c.deadline(ctx)); err != nil {
					return nil, mapTransportError(err, op, c.addr, true)
				}
				packet, err = transport.Receive(ctx, c.conn, c.maxPacketSize, c.deadline(ctx))
				if err != nil {
					return nil, mapTransportError(err, op, c.addr, false)
				}
				continue
			}
			if header != expectedHeader {
				return nil, newError(ErrorCodeUnsupported, op, c.addr, fmt.Sprintf("unexpected payload header 0x%X", header), nil)
			}
			return packet, nil
		case protocol.PacketMulti:
			full, err := multipacket.Collect(ctx, c.conn, packet, c.maxPacketSize, c.deadline(ctx))
			if err != nil {
				return nil, mapInternalError(err, op, c.addr)
			}
			packet = full
			continue
		default:
			return nil, newError(ErrorCodePacketHeader, op, c.addr, "unknown packet type", nil)
		}
	}
}

func (c *Client) deadline(ctx context.Context) time.Time {
	deadline := time.Now().Add(c.timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		return ctxDeadline
	}
	return deadline
}

func normalizeAddress(addr string) (*net.UDPAddr, error) {
	trimmed := strings.TrimSpace(addr)
	if trimmed == "" {
		return nil, newError(ErrorCodeAddress, "new_client", addr, "address must not be empty", nil)
	}

	host, port, err := net.SplitHostPort(trimmed)
	if err != nil {
		if strings.Contains(err.Error(), "missing port in address") {
			trimmed = net.JoinHostPort(trimmed, strconv.Itoa(defaultPort))
		} else {
			return nil, newError(ErrorCodeAddress, "new_client", addr, "invalid address", err)
		}
	} else {
		if host == "" {
			return nil, newError(ErrorCodeAddress, "new_client", addr, "host must not be empty", nil)
		}
		if port == "" {
			trimmed = net.JoinHostPort(host, strconv.Itoa(defaultPort))
		}
	}

	udpAddr, err := net.ResolveUDPAddr("udp", trimmed)
	if err != nil {
		return nil, newError(ErrorCodeAddress, "new_client", trimmed, "resolve udp address failed", err)
	}
	return udpAddr, nil
}

func mapTransportError(err error, op string, addr string, writing bool) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return newError(ErrorCodeTimeout, op, addr, "context canceled or timed out", err)
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return newError(ErrorCodeTimeout, op, addr, "network timeout", err)
	}
	if writing {
		return newError(ErrorCodeWrite, op, addr, "udp write failed", err)
	}
	return newError(ErrorCodeRead, op, addr, "udp read failed", err)
}

func mapInternalError(err error, op string, addr string) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, ierrors.ErrPacketHeader):
		return newError(ErrorCodePacketHeader, op, addr, err.Error(), err)
	case errors.Is(err, ierrors.ErrUnsupported):
		return newError(ErrorCodeUnsupported, op, addr, err.Error(), err)
	case errors.Is(err, ierrors.ErrChallenge):
		return newError(ErrorCodeChallenge, op, addr, err.Error(), err)
	case errors.Is(err, ierrors.ErrDecode):
		return newError(ErrorCodeDecode, op, addr, err.Error(), err)
	case errors.Is(err, ierrors.ErrMultiPacket):
		return newError(ErrorCodeMultiPacket, op, addr, err.Error(), err)
	default:
		return newError(ErrorCodeDecode, op, addr, err.Error(), err)
	}
}
