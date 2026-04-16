package multipacket

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	ierrors "github.com/GoFurry/a2s-go/internal/errors"
)

func TestParseHeader(t *testing.T) {
	t.Parallel()

	packet := buildSplitPacket(compressedFlag|0x01020304, 3, 1, 1248, []byte("payload"))
	header, err := parseHeader(packet)
	if err != nil {
		t.Fatalf("parseHeader returned error: %v", err)
	}

	if got, want := header.ID, uint32(compressedFlag|0x01020304); got != want {
		t.Fatalf("header.ID = 0x%X, want 0x%X", got, want)
	}
	if got, want := header.Total, uint8(3); got != want {
		t.Fatalf("header.Total = %d, want %d", got, want)
	}
	if got, want := header.Number, uint8(1); got != want {
		t.Fatalf("header.Number = %d, want %d", got, want)
	}
	if got, want := header.SplitSize, uint16(1248); got != want {
		t.Fatalf("header.SplitSize = %d, want %d", got, want)
	}
	if !header.Compressed {
		t.Fatal("header.Compressed = false, want true")
	}
	if got, want := header.Payload, []byte("payload"); !bytes.Equal(got, want) {
		t.Fatalf("header.Payload = %q, want %q", got, want)
	}
}

func TestParseHeaderRejectsInvalidPackets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		packet  []byte
		wantErr error
	}{
		{
			name:    "bad signature",
			packet:  []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x00},
			wantErr: ierrors.ErrPacketHeader,
		},
		{
			name:    "truncated header",
			packet:  []byte{0xFE, 0xFF, 0xFF, 0xFF, 0x04, 0x03, 0x02, 0x01},
			wantErr: ierrors.ErrMultiPacket,
		},
		{
			name:    "missing payload",
			packet:  buildSplitPacket(0x01020304, 2, 0, 1248, nil),
			wantErr: ierrors.ErrMultiPacket,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseHeader(tt.packet)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("parseHeader error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateCompatible(t *testing.T) {
	t.Parallel()

	expected := &header{ID: 0x01020304, Total: 3, SplitSize: 1248}
	tests := []struct {
		name    string
		actual  *header
		wantErr string
	}{
		{
			name:   "matching",
			actual: &header{ID: 0x01020304, Total: 3, SplitSize: 1248},
		},
		{
			name:    "id mismatch",
			actual:  &header{ID: 0xFFFFFFFF, Total: 3, SplitSize: 1248},
			wantErr: "packet id mismatch",
		},
		{
			name:    "total mismatch",
			actual:  &header{ID: 0x01020304, Total: 2, SplitSize: 1248},
			wantErr: "packet total mismatch",
		},
		{
			name:    "split size mismatch",
			actual:  &header{ID: 0x01020304, Total: 3, SplitSize: 1400},
			wantErr: "packet split size mismatch",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateCompatible(expected, tt.actual)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateCompatible returned error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validateCompatible error = %v, want substring %q", err, tt.wantErr)
			}
			if !errors.Is(err, ierrors.ErrMultiPacket) {
				t.Fatalf("validateCompatible error = %v, want ErrMultiPacket", err)
			}
		})
	}
}

func TestCollectAssemblesUncompressedSplitPacket(t *testing.T) {
	clientConn := mustListenUDP(t)
	defer clientConn.Close()

	serverConn := mustListenUDP(t)
	defer serverConn.Close()

	payload := []byte("hello from a split packet response")
	first := buildSplitPacket(0x11223344, 2, 0, 1248, payload[:12])
	second := buildSplitPacket(0x11223344, 2, 1, 1248, payload[12:])

	writePackets(t, serverConn, clientConn.LocalAddr().(*net.UDPAddr), second)

	packet, err := Collect(
		context.Background(),
		clientConn,
		serverConn.LocalAddr().(*net.UDPAddr),
		first,
		4096,
		time.Now().Add(time.Second),
	)
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	if got, want := packet, payload; !bytes.Equal(got, want) {
		t.Fatalf("packet = %q, want %q", got, want)
	}
}

func TestCollectIgnoresUnexpectedSourceFragments(t *testing.T) {
	clientConn := mustListenUDP(t)
	defer clientConn.Close()

	serverConn := mustListenUDP(t)
	defer serverConn.Close()

	noiseConn := mustListenUDP(t)
	defer noiseConn.Close()

	payload := []byte("split response that should ignore foreign fragments")
	first := buildSplitPacket(0x55667788, 2, 0, 1248, payload[:20])
	goodSecond := buildSplitPacket(0x55667788, 2, 1, 1248, payload[20:])
	badSecond := buildSplitPacket(0x55667788, 2, 1, 1248, []byte("foreign payload"))

	target := clientConn.LocalAddr().(*net.UDPAddr)
	writePackets(t, noiseConn, target, badSecond)
	writePackets(t, serverConn, target, goodSecond)

	packet, err := Collect(
		context.Background(),
		clientConn,
		serverConn.LocalAddr().(*net.UDPAddr),
		first,
		4096,
		time.Now().Add(time.Second),
	)
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	if got, want := packet, payload; !bytes.Equal(got, want) {
		t.Fatalf("packet = %q, want %q", got, want)
	}
}

func TestCollectRejectsDuplicatePacketIndex(t *testing.T) {
	clientConn := mustListenUDP(t)
	defer clientConn.Close()

	serverConn := mustListenUDP(t)
	defer serverConn.Close()

	first := buildSplitPacket(0x99AABBCC, 2, 0, 1248, []byte("first"))
	duplicate := buildSplitPacket(0x99AABBCC, 2, 0, 1248, []byte("second"))

	writePackets(t, serverConn, clientConn.LocalAddr().(*net.UDPAddr), duplicate)

	_, err := Collect(
		context.Background(),
		clientConn,
		serverConn.LocalAddr().(*net.UDPAddr),
		first,
		4096,
		time.Now().Add(time.Second),
	)
	if err == nil || !strings.Contains(err.Error(), "duplicate packet index") {
		t.Fatalf("Collect error = %v, want duplicate packet index", err)
	}
	if !errors.Is(err, ierrors.ErrMultiPacket) {
		t.Fatalf("Collect error = %v, want ErrMultiPacket", err)
	}
}

func TestCollectAssemblesCompressedSplitPacket(t *testing.T) {
	clientConn := mustListenUDP(t)
	defer clientConn.Close()

	serverConn := mustListenUDP(t)
	defer serverConn.Close()

	envelope := buildCompressedEnvelope(compressedFixturePayload, compressedFixtureBzip2)
	first := buildSplitPacket(compressedFlag|0x01020304, 3, 0, 1248, envelope[:18])
	second := buildSplitPacket(compressedFlag|0x01020304, 3, 1, 1248, envelope[18:45])
	third := buildSplitPacket(compressedFlag|0x01020304, 3, 2, 1248, envelope[45:])

	writePackets(t, serverConn, clientConn.LocalAddr().(*net.UDPAddr), second, third)

	packet, err := Collect(
		context.Background(),
		clientConn,
		serverConn.LocalAddr().(*net.UDPAddr),
		first,
		4096,
		time.Now().Add(time.Second),
	)
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	if got, want := packet, compressedFixturePayload; !bytes.Equal(got, want) {
		t.Fatalf("packet = %q, want %q", got, want)
	}
}

func TestCollectRejectsChecksumMismatch(t *testing.T) {
	t.Parallel()

	envelope := buildCompressedEnvelope(compressedFixturePayload, compressedFixtureBzip2)
	bad := append([]byte(nil), envelope...)
	binary.LittleEndian.PutUint32(bad[4:8], 0xDEADBEEF)

	packet := buildSplitPacket(compressedFlag|0x0BADF00D, 1, 0, 1248, bad)
	_, err := Collect(context.Background(), nil, nil, packet, 4096, time.Time{})
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("Collect error = %v, want checksum mismatch", err)
	}
	if !errors.Is(err, ierrors.ErrMultiPacket) {
		t.Fatalf("Collect error = %v, want ErrMultiPacket", err)
	}
}

func TestReadBzip2RejectsOversizedPayloads(t *testing.T) {
	t.Parallel()

	_, err := readBzip2(compressedFixtureBzip2, maxDecompressedPayloadSize+1)
	if err == nil || !strings.Contains(err.Error(), "exceeds limit") {
		t.Fatalf("readBzip2 error = %v, want size limit failure", err)
	}
}
