package transport

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestReceiveFromIgnoresUnexpectedSource(t *testing.T) {
	t.Parallel()

	clientConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("ListenUDP client returned error: %v", err)
	}
	defer clientConn.Close()

	expectedConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("ListenUDP expected returned error: %v", err)
	}
	defer expectedConn.Close()

	noiseConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("ListenUDP noise returned error: %v", err)
	}
	defer noiseConn.Close()

	target := clientConn.LocalAddr().(*net.UDPAddr)
	expected := expectedConn.LocalAddr().(*net.UDPAddr)

	if _, err := noiseConn.WriteToUDP([]byte("noise"), target); err != nil {
		t.Fatalf("noise WriteToUDP returned error: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if _, err := expectedConn.WriteToUDP([]byte("good"), target); err != nil {
		t.Fatalf("expected WriteToUDP returned error: %v", err)
	}

	packet, err := ReceiveFrom(context.Background(), clientConn, expected, 32, time.Now().Add(time.Second))
	if err != nil {
		t.Fatalf("ReceiveFrom returned error: %v", err)
	}
	if got, want := string(packet), "good"; got != want {
		t.Fatalf("packet = %q, want %q", got, want)
	}
}

func BenchmarkAcquireReleaseReadBuffer(b *testing.B) {
	for i := 0; i < b.N; i++ {
		buf := acquireReadBuffer(4096)
		releaseReadBuffer(buf)
	}
}
