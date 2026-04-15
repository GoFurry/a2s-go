package master

import (
	"fmt"
	"net"
	"strconv"

	"github.com/GoFurry/a2s-go/internal/masterprotocol"
)

var zeroIPv4 = net.IPv4zero.To4()

type cursorState uint8

const (
	cursorStateZero cursorState = iota
	cursorStateNormal
	cursorStateTerminal
)

// Request describes one master/discovery page query.
type Request struct {
	Region Region
	Filter string
	Cursor Cursor
}

// Page is one discovery page.
type Page struct {
	Servers    []ServerAddr
	NextCursor Cursor
	Done       bool
}

// Result is one streamed server address or a terminal page error.
type Result struct {
	Server ServerAddr
	Err    error
}

// ServerAddr is one discovered game server address.
type ServerAddr struct {
	IP   net.IP
	Port uint16
}

func (a ServerAddr) String() string {
	ip := a.IP.To4()
	if ip == nil {
		ip = zeroIPv4
	}
	return net.JoinHostPort(ip.String(), strconv.Itoa(int(a.Port)))
}

func (a ServerAddr) toProtocol() masterprotocol.ServerAddr {
	return masterprotocol.ServerAddr{
		IP:   cloneIPv4(a.IP),
		Port: a.Port,
	}
}

func serverAddrFromProtocol(addr masterprotocol.ServerAddr) ServerAddr {
	return ServerAddr{
		IP:   cloneIPv4(addr.IP),
		Port: addr.Port,
	}
}

func zeroServerAddr() ServerAddr {
	return ServerAddr{
		IP:   cloneIPv4(zeroIPv4),
		Port: 0,
	}
}

// Cursor is the public discovery pagination token.
type Cursor struct {
	state cursorState
	addr  ServerAddr
}

// StartCursor returns the beginning cursor.
func StartCursor() Cursor {
	return Cursor{}
}

func terminalCursor() Cursor {
	return Cursor{state: cursorStateTerminal}
}

func cursorFromServerAddr(addr ServerAddr) Cursor {
	return Cursor{
		state: cursorStateNormal,
		addr:  addr,
	}
}

// IsZero reports whether the cursor is the start cursor.
func (c Cursor) IsZero() bool {
	return c.state == cursorStateZero
}

// IsTerminal reports whether the cursor is the end cursor.
func (c Cursor) IsTerminal() bool {
	return c.state == cursorStateTerminal
}

// String returns a stable host:port representation.
func (c Cursor) String() string {
	if c.state != cursorStateNormal {
		return zeroServerAddr().String()
	}
	return c.addr.String()
}

func (c Cursor) seedAddr() ServerAddr {
	if c.state != cursorStateNormal {
		return zeroServerAddr()
	}
	return c.addr
}

// Region identifies a master server region code.
type Region byte

const (
	RegionUSEast       Region = 0x00
	RegionUSWest       Region = 0x01
	RegionSouthAmerica Region = 0x02
	RegionEurope       Region = 0x03
	RegionAsia         Region = 0x04
	RegionAustralia    Region = 0x05
	RegionMiddleEast   Region = 0x06
	RegionAfrica       Region = 0x07
	RegionRestOfWorld  Region = 0xFF
)

// RegionCustom keeps the door open for additional protocol values.
func RegionCustom(value byte) Region {
	return Region(value)
}

func (r Region) String() string {
	switch r {
	case RegionUSEast:
		return "us_east"
	case RegionUSWest:
		return "us_west"
	case RegionSouthAmerica:
		return "south_america"
	case RegionEurope:
		return "europe"
	case RegionAsia:
		return "asia"
	case RegionAustralia:
		return "australia"
	case RegionMiddleEast:
		return "middle_east"
	case RegionAfrica:
		return "africa"
	case RegionRestOfWorld:
		return "rest_of_world"
	default:
		return fmt.Sprintf("custom(0x%X)", byte(r))
	}
}

func cloneIPv4(ip net.IP) net.IP {
	ip = ip.To4()
	if ip == nil {
		return append(net.IP(nil), zeroIPv4...)
	}
	return append(net.IP(nil), ip...)
}
