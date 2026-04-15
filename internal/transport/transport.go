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

// SendTo writes one UDP packet to a target address with deadline handling.
func SendTo(ctx context.Context, conn *net.UDPConn, addr *net.UDPAddr, packet []byte, deadline time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := conn.SetWriteDeadline(deadline); err != nil {
		return err
	}
	_, err := conn.WriteToUDP(packet, addr)
	return err
}

// ReceiveFrom reads one UDP packet from the expected address with deadline handling.
func ReceiveFrom(ctx context.Context, conn *net.UDPConn, expected *net.UDPAddr, maxPacketSize int, deadline time.Time) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	buf := make([]byte, maxPacketSize)
	for {
		if err := conn.SetReadDeadline(deadline); err != nil {
			return nil, err
		}
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			return nil, err
		}
		if sameUDPAddr(addr, expected) {
			out := make([]byte, n)
			copy(out, buf[:n])
			return out, nil
		}
	}
}

func sameUDPAddr(left *net.UDPAddr, right *net.UDPAddr) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Port == right.Port && left.IP.Equal(right.IP) && left.Zone == right.Zone
}
