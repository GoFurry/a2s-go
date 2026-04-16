package protocol

import (
	"bytes"
	"encoding/binary"
	"math"
	"net"

	ierrors "github.com/GoFurry/a2s-go/internal/errors"
)

const infoQueryString = "Source Engine Query"
const infoRequestBaseLen = 4 + 1 + len(infoQueryString) + 1

type packetBuilder struct {
	bytes.Buffer
}

func (b *packetBuilder) writeCString(s string) {
	b.WriteString(s)
	b.WriteByte(0)
}

// Reader reads A2S packets with little-endian semantics.
type Reader struct {
	buf []byte
	pos int
}

// NewReader creates a packet reader.
func NewReader(buf []byte) *Reader {
	return &Reader{buf: buf}
}

// Pos returns current offset.
func (r *Reader) Pos() int {
	return r.pos
}

// Remaining reports if unread bytes remain.
func (r *Reader) Remaining() bool {
	return r.pos < len(r.buf)
}

func (r *Reader) canRead(size int) bool {
	return r.pos+size <= len(r.buf)
}

func (r *Reader) Uint8() (uint8, bool) {
	if !r.canRead(1) {
		return 0, false
	}
	v := r.buf[r.pos]
	r.pos++
	return v, true
}

func (r *Reader) Uint16() (uint16, bool) {
	if !r.canRead(2) {
		return 0, false
	}
	v := binary.LittleEndian.Uint16(r.buf[r.pos:])
	r.pos += 2
	return v, true
}

func (r *Reader) Uint32() (uint32, bool) {
	if !r.canRead(4) {
		return 0, false
	}
	v := binary.LittleEndian.Uint32(r.buf[r.pos:])
	r.pos += 4
	return v, true
}

func (r *Reader) Int32() (int32, bool) {
	v, ok := r.Uint32()
	return int32(v), ok
}

func (r *Reader) Uint64() (uint64, bool) {
	if !r.canRead(8) {
		return 0, false
	}
	v := binary.LittleEndian.Uint64(r.buf[r.pos:])
	r.pos += 8
	return v, true
}

func (r *Reader) Float32() (float32, bool) {
	v, ok := r.Uint32()
	if !ok {
		return 0, false
	}
	return math.Float32frombits(v), true
}

func (r *Reader) String() (string, bool) {
	start := r.pos
	for r.pos < len(r.buf) {
		if r.buf[r.pos] == 0 {
			s := string(r.buf[start:r.pos])
			r.pos++
			return s, true
		}
		r.pos++
	}
	return "", false
}

func (r *Reader) IPv4() (net.IP, bool) {
	if !r.canRead(net.IPv4len) {
		return nil, false
	}
	ip := net.IP(r.buf[r.pos : r.pos+net.IPv4len])
	r.pos += net.IPv4len
	return ip, true
}

func (r *Reader) PortBE() (uint16, bool) {
	if !r.canRead(2) {
		return 0, false
	}
	v := binary.BigEndian.Uint16(r.buf[r.pos:])
	r.pos += 2
	return v, true
}

// BuildInfoRequest builds an A2S_INFO request.
func BuildInfoRequest() []byte {
	var b packetBuilder
	b.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF, RequestInfo})
	b.writeCString(infoQueryString)
	return b.Bytes()
}

// BuildPlayersRequest builds an A2S_PLAYER request.
func BuildPlayersRequest(challenge uint32) []byte {
	return buildQueryWithChallenge(RequestPlayers, challenge)
}

// BuildRulesRequest builds an A2S_RULES request.
func BuildRulesRequest(challenge uint32) []byte {
	return buildQueryWithChallenge(RequestRules, challenge)
}

func buildQueryWithChallenge(kind byte, challenge uint32) []byte {
	buf := make([]byte, 9)
	copy(buf, []byte{0xFF, 0xFF, 0xFF, 0xFF, kind})
	binary.LittleEndian.PutUint32(buf[5:], challenge)
	return buf
}

// ApplyChallenge replaces the challenge field in a request.
func ApplyChallenge(request []byte, challenge uint32) []byte {
	if len(request) < 5 {
		return request
	}
	if request[4] == RequestInfo {
		out := append([]byte(nil), request...)
		if len(out) >= infoRequestBaseLen+4 {
			binary.LittleEndian.PutUint32(out[len(out)-4:], challenge)
			return out
		}
		var token [4]byte
		binary.LittleEndian.PutUint32(token[:], challenge)
		out = append(out, token[:]...)
		return out
	}
	if len(request) < 9 {
		return request
	}
	out := append([]byte(nil), request...)
	binary.LittleEndian.PutUint32(out[5:], challenge)
	return out
}

// ClassifyPacket identifies whether the packet is single or split.
func ClassifyPacket(packet []byte) (PacketKind, error) {
	r := NewReader(packet)
	header, ok := r.Int32()
	if !ok {
		return PacketUnknown, ierrors.ErrPacketHeader
	}
	switch header {
	case packetSingle:
		return PacketSingle, nil
	case packetMulti:
		return PacketMulti, nil
	default:
		return PacketUnknown, ierrors.ErrPacketHeader
	}
}

// PeekPayloadHeader returns the protocol header after the framing header.
func PeekPayloadHeader(packet []byte) (byte, error) {
	r := NewReader(packet)
	if _, ok := r.Int32(); !ok {
		return 0, ierrors.ErrPacketHeader
	}
	header, ok := r.Uint8()
	if !ok {
		return 0, ierrors.ErrPacketHeader
	}
	return header, nil
}
