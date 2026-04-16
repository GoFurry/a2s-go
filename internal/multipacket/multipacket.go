package multipacket

import (
	"bytes"
	"compress/bzip2"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"time"

	ierrors "github.com/GoFurry/a2s-go/internal/errors"
	"github.com/GoFurry/a2s-go/internal/protocol"
	"github.com/GoFurry/a2s-go/internal/transport"
)

const compressedFlag = 0x80000000
const maxDecompressedPayloadSize = 2 * 1024 * 1024

type header struct {
	ID         uint32
	Total      uint8
	Number     uint8
	SplitSize  uint16
	Compressed bool
	Payload    []byte
}

// Collect reads and assembles a split packet response.
func Collect(ctx context.Context, conn *net.UDPConn, expectedSource *net.UDPAddr, first []byte, maxPacketSize int, deadline time.Time) ([]byte, error) {
	h, err := parseHeader(first)
	if err != nil {
		return nil, err
	}
	expected := *h
	parts := make([]*header, int(h.Total))
	received := 0
	totalSize := 0

	for {
		if int(h.Number) >= len(parts) {
			return nil, errors.Join(ierrors.ErrMultiPacket, errors.New("packet index out of bounds"))
		}
		if parts[h.Number] != nil {
			return nil, errors.Join(ierrors.ErrMultiPacket, errors.New("duplicate packet index"))
		}
		parts[h.Number] = h
		totalSize += len(h.Payload)
		received++
		if received == len(parts) {
			break
		}

		packet, err := receiveNext(ctx, conn, expectedSource, maxPacketSize, deadline)
		if err != nil {
			return nil, err
		}
		h, err = parseHeader(packet)
		if err != nil {
			return nil, err
		}
		if err := validateCompatible(&expected, h); err != nil {
			return nil, err
		}
	}

	payload := make([]byte, totalSize)
	offset := 0
	for _, part := range parts {
		copy(payload[offset:], part.Payload)
		offset += len(part.Payload)
	}

	if len(parts) == 0 || !parts[0].Compressed {
		return payload, nil
	}

	if len(payload) < 8 {
		return nil, errors.Join(ierrors.ErrMultiPacket, errors.New("compressed multi packet payload too small"))
	}

	decompressedSize := binary.LittleEndian.Uint32(payload[:4])
	checksum := binary.LittleEndian.Uint32(payload[4:8])
	decompressed, err := readBzip2(payload[8:], int(decompressedSize))
	if err != nil {
		return nil, errors.Join(ierrors.ErrMultiPacket, err)
	}
	if crc32.ChecksumIEEE(decompressed) != checksum {
		return nil, errors.Join(ierrors.ErrMultiPacket, errors.New("checksum mismatch"))
	}
	return decompressed, nil
}

func receiveNext(ctx context.Context, conn *net.UDPConn, expectedSource *net.UDPAddr, maxPacketSize int, deadline time.Time) ([]byte, error) {
	if expectedSource == nil {
		return transport.Receive(ctx, conn, maxPacketSize, deadline)
	}
	return transport.ReceiveFrom(ctx, conn, expectedSource, maxPacketSize, deadline)
}

func validateCompatible(expected *header, actual *header) error {
	switch {
	case actual.ID != expected.ID:
		return errors.Join(ierrors.ErrMultiPacket, errors.New("packet id mismatch"))
	case actual.Total != expected.Total:
		return errors.Join(ierrors.ErrMultiPacket, errors.New("packet total mismatch"))
	case actual.SplitSize != expected.SplitSize:
		return errors.Join(ierrors.ErrMultiPacket, errors.New("packet split size mismatch"))
	default:
		return nil
	}
}

func parseHeader(packet []byte) (*header, error) {
	r := protocol.NewReader(packet)
	if signature, ok := r.Int32(); !ok || signature != -2 {
		return nil, ierrors.ErrPacketHeader
	}

	id, ok := r.Uint32()
	if !ok {
		return nil, ierrors.ErrMultiPacket
	}
	total, ok := r.Uint8()
	if !ok {
		return nil, ierrors.ErrMultiPacket
	}
	number, ok := r.Uint8()
	if !ok {
		return nil, ierrors.ErrMultiPacket
	}
	splitSize, ok := r.Uint16()
	if !ok {
		return nil, ierrors.ErrMultiPacket
	}
	if !r.Remaining() {
		return nil, ierrors.ErrMultiPacket
	}

	return &header{
		ID:         id,
		Total:      total,
		Number:     number,
		SplitSize:  splitSize,
		Compressed: id&compressedFlag != 0,
		Payload:    packet[r.Pos():],
	}, nil
}

func readBzip2(payload []byte, size int) ([]byte, error) {
	if size < 0 {
		return nil, errors.New("invalid decompressed size")
	}
	if size > maxDecompressedPayloadSize {
		return nil, fmt.Errorf("declared decompressed size %d exceeds limit %d", size, maxDecompressedPayloadSize)
	}

	reader := io.LimitReader(bzip2.NewReader(bytes.NewReader(payload)), int64(size)+1)
	out, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if len(out) != size {
		return nil, errors.New("decompressed size mismatch")
	}
	return out, nil
}
