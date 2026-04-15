package transport

import (
	"context"
	"net"
	"time"
)

// Send writes one UDP packet with deadline handling.
func Send(ctx context.Context, conn *net.UDPConn, packet []byte, deadline time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := conn.SetWriteDeadline(deadline); err != nil {
		return err
	}
	_, err := conn.Write(packet)
	return err
}

// Receive reads one UDP packet with deadline handling.
func Receive(ctx context.Context, conn *net.UDPConn, maxPacketSize int, deadline time.Time) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := conn.SetReadDeadline(deadline); err != nil {
		return nil, err
	}
	buf := make([]byte, maxPacketSize)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}
	out := make([]byte, n)
	copy(out, buf[:n])
	return out, nil
}
