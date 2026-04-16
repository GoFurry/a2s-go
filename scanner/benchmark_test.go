package scanner

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/GoFurry/a2s-go/master"
)

func BenchmarkCollectInfo(b *testing.B) {
	var servers []*benchmarkInfoServer
	var inputs []master.ServerAddr
	for i := 0; i < 16; i++ {
		server, err := newBenchmarkInfoServer()
		if err != nil {
			b.Fatalf("newBenchmarkInfoServer returned error: %v", err)
		}
		servers = append(servers, server)
		inputs = append(inputs, server.ServerAddr())
	}
	defer func() {
		for _, server := range servers {
			server.Close()
		}
	}()

	client, err := NewClient(WithConcurrency(8))
	if err != nil {
		b.Fatalf("NewClient returned error: %v", err)
	}

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := client.CollectInfo(ctx, Request{Servers: inputs})
		if err != nil {
			b.Fatalf("CollectInfo returned error: %v", err)
		}
		if got, want := len(results), len(inputs); got != want {
			b.Fatalf("len(results) = %d, want %d", got, want)
		}
	}
}

type benchmarkInfoServer struct {
	conn *net.UDPConn
	done chan struct{}
}

func newBenchmarkInfoServer() (*benchmarkInfoServer, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		return nil, err
	}

	server := &benchmarkInfoServer{
		conn: conn,
		done: make(chan struct{}),
	}
	go server.serve()
	return server, nil
}

func (s *benchmarkInfoServer) ServerAddr() master.ServerAddr {
	addr := s.conn.LocalAddr().(*net.UDPAddr)
	return master.ServerAddr{
		IP:   append(net.IP(nil), addr.IP.To4()...),
		Port: uint16(addr.Port),
	}
}

func (s *benchmarkInfoServer) Close() {
	close(s.done)
	_ = s.conn.Close()
}

func (s *benchmarkInfoServer) serve() {
	buf := make([]byte, 4096)
	response := buildBenchmarkInfoResponse()
	for {
		_ = s.conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		n, remote, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				select {
				case <-s.done:
					return
				default:
					continue
				}
			}
			return
		}
		if !bytes.Equal(buf[:n], buildInfoRequestBytes()) {
			continue
		}
		if _, err := s.conn.WriteToUDP(response, remote); err != nil {
			return
		}
	}
}

func buildBenchmarkInfoResponse() []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x49, 17})
	writeCString(&buf, "bench")
	writeCString(&buf, "cp_badlands")
	writeCString(&buf, "tf")
	writeCString(&buf, "Team Fortress 2")
	_ = binary.Write(&buf, binary.LittleEndian, uint16(440))
	buf.WriteByte(12)
	buf.WriteByte(24)
	buf.WriteByte(0)
	buf.WriteByte('d')
	buf.WriteByte('l')
	buf.WriteByte(0)
	buf.WriteByte(1)
	writeCString(&buf, "1.0.0")
	buf.WriteByte(0xB1)
	_ = binary.Write(&buf, binary.LittleEndian, uint16(27015))
	_ = binary.Write(&buf, binary.LittleEndian, uint64(76561198000000000))
	writeCString(&buf, "bench")
	_ = binary.Write(&buf, binary.LittleEndian, uint64(440))
	return buf.Bytes()
}
