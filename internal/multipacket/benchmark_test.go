package multipacket

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"
)

func BenchmarkCollectUncompressed(b *testing.B) {
	payload := []byte("hello from a split packet response")
	packets := [][]byte{
		buildSplitPacket(0x11223344, 2, 0, 1248, payload[:12]),
		buildSplitPacket(0x11223344, 2, 1, 1248, payload[12:]),
	}
	benchmarkCollect(b, packets, payload)
}

func BenchmarkCollectCompressed(b *testing.B) {
	envelope := buildCompressedEnvelope(compressedFixturePayload, compressedFixtureBzip2)
	packets := [][]byte{
		buildSplitPacket(compressedFlag|0x01020304, 3, 0, 1248, envelope[:18]),
		buildSplitPacket(compressedFlag|0x01020304, 3, 1, 1248, envelope[18:45]),
		buildSplitPacket(compressedFlag|0x01020304, 3, 2, 1248, envelope[45:]),
	}
	benchmarkCollect(b, packets, compressedFixturePayload)
}

func BenchmarkReadBzip2(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		payload, err := readBzip2(compressedFixtureBzip2, len(compressedFixturePayload))
		if err != nil {
			b.Fatalf("readBzip2 returned error: %v", err)
		}
		if !bytes.Equal(payload, compressedFixturePayload) {
			b.Fatalf("payload = %q, want %q", payload, compressedFixturePayload)
		}
	}
}

func benchmarkCollect(b *testing.B, packets [][]byte, want []byte) {
	clientConn := mustListenUDP(b)
	defer clientConn.Close()

	serverConn := mustListenUDP(b)
	defer serverConn.Close()

	target := clientConn.LocalAddr().(*net.UDPAddr)
	expectedSource := serverConn.LocalAddr().(*net.UDPAddr)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writePackets(b, serverConn, target, packets[1:]...)
		packet, err := Collect(ctx, clientConn, expectedSource, packets[0], 4096, time.Now().Add(time.Second))
		if err != nil {
			b.Fatalf("Collect returned error: %v", err)
		}
		if !bytes.Equal(packet, want) {
			b.Fatalf("packet = %q, want %q", packet, want)
		}
	}
}
