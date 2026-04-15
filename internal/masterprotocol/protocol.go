package masterprotocol

import (
	"bytes"
	"net"
	"strconv"

	ierrors "github.com/GoFurry/a2s-go/internal/errors"
	"github.com/GoFurry/a2s-go/internal/protocol"
)

const (
	requestType   = 0x31
	responseType  = 0x66
	responseTrail = 0x0A
)

var zeroIPv4 = net.IPv4zero.To4()

// ServerAddr is the internal master-protocol address shape.
type ServerAddr struct {
	IP   net.IP
	Port uint16
}

// Page is one decoded discovery response page.
type Page struct {
	Servers []ServerAddr
	Done    bool
}

// BuildQuery constructs one master server discovery request.
func BuildQuery(region byte, cursor ServerAddr, filter string) []byte {
	var buf bytes.Buffer
	buf.WriteByte(requestType)
	buf.WriteByte(region)
	buf.WriteString(formatAddr(cursor))
	buf.WriteByte(0)
	buf.WriteString(filter)
	buf.WriteByte(0)
	return buf.Bytes()
}

// DecodeResponse parses a master server reply into one page of addresses.
func DecodeResponse(packet []byte) (*Page, error) {
	if len(packet) < 6 {
		return nil, ierrors.ErrPacketHeader
	}
	if !bytes.Equal(packet[:4], []byte{0xFF, 0xFF, 0xFF, 0xFF}) {
		return nil, ierrors.ErrPacketHeader
	}
	if packet[4] != responseType || packet[5] != responseTrail {
		return nil, ierrors.ErrPacketHeader
	}

	r := protocol.NewReader(packet[6:])
	page := &Page{}
	for r.Remaining() {
		ip, ok := r.IPv4()
		if !ok {
			return nil, ierrors.ErrDecode
		}
		port, ok := r.PortBE()
		if !ok {
			return nil, ierrors.ErrDecode
		}

		addr := ServerAddr{
			IP:   cloneIPv4(ip),
			Port: port,
		}
		if IsTerminal(addr) {
			page.Done = true
			if r.Remaining() {
				return nil, ierrors.ErrDecode
			}
			return page, nil
		}
		page.Servers = append(page.Servers, addr)
	}

	return page, nil
}

// IsTerminal reports whether the address is the protocol terminator.
func IsTerminal(addr ServerAddr) bool {
	ip := addr.IP.To4()
	return addr.Port == 0 && ip != nil && ip.Equal(zeroIPv4)
}

func formatAddr(addr ServerAddr) string {
	ip := addr.IP.To4()
	if ip == nil {
		ip = zeroIPv4
	}
	return net.JoinHostPort(ip.String(), strconv.Itoa(int(addr.Port)))
}

func cloneIPv4(ip net.IP) net.IP {
	ip = ip.To4()
	if ip == nil {
		return cloneIPv4(zeroIPv4)
	}
	return append(net.IP(nil), ip...)
}
