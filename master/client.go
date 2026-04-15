package master

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	ierrors "github.com/GoFurry/a2s-go/internal/errors"
	"github.com/GoFurry/a2s-go/internal/masterprotocol"
	"github.com/GoFurry/a2s-go/internal/transport"
)

// Client queries one Valve master server over UDP.
type Client struct {
	addr          string
	timeout       time.Duration
	maxPacketSize int

	conn   *net.UDPConn
	closed bool
	mu     sync.Mutex
}

// NewClient creates one discovery client.
func NewClient(opts ...Option) (*Client, error) {
	cfg := defaultClientConfig()
	for _, opt := range opts {
		if opt == nil {
			return nil, newError(ErrorCodeAddress, "new_client", "", "option must not be nil", nil)
		}
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	normalized, err := normalizeAddress(cfg.baseAddress)
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

// Query requests exactly one discovery page.
func (c *Client) Query(ctx context.Context, req Request) (*Page, error) {
	if err := c.ensureUsable(); err != nil {
		return nil, err
	}
	if err := validateRequest(req); err != nil {
		return nil, err
	}
	if req.Cursor.IsTerminal() {
		return &Page{
			NextCursor: req.Cursor,
			Done:       true,
		}, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	packet := masterprotocol.BuildQuery(byte(req.Region), req.Cursor.seedAddr().toProtocol(), req.Filter)
	if err := transport.Send(ctx, c.conn, packet, c.deadline(ctx)); err != nil {
		return nil, mapTransportError(err, "query", c.addr, true)
	}

	response, err := transport.Receive(ctx, c.conn, c.maxPacketSize, c.deadline(ctx))
	if err != nil {
		return nil, mapTransportError(err, "query", c.addr, false)
	}

	decoded, err := masterprotocol.DecodeResponse(response)
	if err != nil {
		return nil, mapProtocolError(err, "query", c.addr)
	}

	page := &Page{
		Servers: make([]ServerAddr, 0, len(decoded.Servers)),
		Done:    decoded.Done,
	}
	for _, server := range decoded.Servers {
		page.Servers = append(page.Servers, serverAddrFromProtocol(server))
	}
	if decoded.Done {
		page.NextCursor = terminalCursor()
	} else if len(page.Servers) == 0 {
		page.NextCursor = req.Cursor
	} else {
		page.NextCursor = cursorFromServerAddr(page.Servers[len(page.Servers)-1])
	}

	return page, nil
}

// Stream continuously queries pages and emits one address at a time.
func (c *Client) Stream(ctx context.Context, req Request) (<-chan Result, error) {
	if err := c.ensureUsable(); err != nil {
		return nil, err
	}
	if err := validateRequest(req); err != nil {
		return nil, err
	}

	out := make(chan Result)
	go func() {
		defer close(out)

		current := req
		for {
			page, err := c.Query(ctx, current)
			if err != nil {
				select {
				case out <- Result{Err: err}:
				case <-ctx.Done():
				}
				return
			}

			for _, server := range page.Servers {
				select {
				case out <- Result{Server: server}:
				case <-ctx.Done():
					return
				}
			}

			if page.Done {
				return
			}
			current.Cursor = page.NextCursor
		}
	}()

	return out, nil
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

func (c *Client) ensureUsable() error {
	if c == nil {
		return newError(ErrorCodeDial, "ensure_usable", "", "client is nil", nil)
	}
	if c.closed {
		return newError(ErrorCodeDial, "ensure_usable", c.addr, "client is closed", nil)
	}
	return nil
}

func (c *Client) deadline(ctx context.Context) time.Time {
	deadline := time.Now().Add(c.timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		return ctxDeadline
	}
	return deadline
}

func validateRequest(req Request) error {
	if strings.ContainsRune(req.Filter, '\x00') {
		return newError(ErrorCodeFilter, "validate_request", "", "filter must not contain NUL bytes", nil)
	}
	return nil
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

func mapProtocolError(err error, op string, addr string) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, ierrors.ErrPacketHeader):
		return newError(ErrorCodePacketHeader, op, addr, err.Error(), err)
	case errors.Is(err, ierrors.ErrDecode):
		return newError(ErrorCodeDecode, op, addr, err.Error(), err)
	default:
		return newError(ErrorCodeDecode, op, addr, err.Error(), err)
	}
}
