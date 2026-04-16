package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"net"
	"time"

	"github.com/GoFurry/a2s-go/master"
	"github.com/GoFurry/a2s-go/scanner"
)

const noChallenge = ^uint32(0)

func main() {
	serverA, err := newFakeGameServer(fakeGameConfig{
		name:      "Scanner Alpha",
		players:   []string{"alpha-one", "alpha-two"},
		rulesHost: "alpha-host",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer serverA.Close()

	serverB, err := newFakeGameServer(fakeGameConfig{
		name:      "Scanner Bravo",
		players:   []string{"bravo-one"},
		rulesHost: "bravo-host",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer serverB.Close()

	scanClient, err := scanner.NewClient(
		scanner.WithConcurrency(2),
		scanner.WithTimeout(3*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	staticTargets, err := scanner.ParseAddresses([]string{
		serverA.ServerAddr().String(),
		serverB.ServerAddr().String(),
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("== Probe fixed servers (info) ==")
	infoResults, err := scanClient.CollectInfo(ctx, scanner.Request{
		Servers: staticTargets,
	})
	if err != nil {
		log.Fatal(err)
	}
	for _, result := range infoResults {
		if result.Err != nil {
			fmt.Printf("info error: %v\n", result.Err)
			continue
		}
		fmt.Printf("%s -> %s\n", result.Server.String(), result.Info.Name)
	}

	fmt.Println()
	fmt.Println("== Probe fixed servers (players) ==")
	playersResults, err := scanClient.CollectPlayers(ctx, scanner.Request{
		Servers: staticTargets,
	})
	if err != nil {
		log.Fatal(err)
	}
	for _, result := range playersResults {
		if result.Err != nil {
			fmt.Printf("players error: %v\n", result.Err)
			continue
		}
		fmt.Printf("%s -> count=%d\n", result.Server.String(), result.Players.Count)
	}

	fmt.Println()
	fmt.Println("== Probe fixed servers (rules) ==")
	rulesResults, err := scanClient.CollectRules(ctx, scanner.Request{
		Servers: staticTargets,
	})
	if err != nil {
		log.Fatal(err)
	}
	for _, result := range rulesResults {
		if result.Err != nil {
			fmt.Printf("rules error: %v\n", result.Err)
			continue
		}
		fmt.Printf("%s -> hostname=%s\n", result.Server.String(), result.Rules.Items["hostname"])
	}

	fmt.Println()
	fmt.Println("== Stream fake master into scanner ==")
	masterServer, err := newFakeMasterServer([]master.ServerAddr{serverA.ServerAddr(), serverB.ServerAddr()})
	if err != nil {
		log.Fatal(err)
	}
	defer masterServer.Close()

	masterClient, err := master.NewClient(
		master.WithBaseAddress(masterServer.Addr().String()),
		master.WithTimeout(3*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer masterClient.Close()

	discovery, err := masterClient.Stream(ctx, master.Request{
		Region: master.RegionAsia,
		Filter: "\\secure\\1",
	})
	if err != nil {
		log.Fatal(err)
	}

	stream, err := scanClient.ProbeInfo(ctx, scanner.Request{Discovery: discovery})
	if err != nil {
		log.Fatal(err)
	}
	for result := range stream {
		if result.Err != nil {
			fmt.Printf("master->scanner error: %v\n", result.Err)
			continue
		}
		fmt.Printf("%s -> %s\n", result.Server.String(), result.Info.Name)
	}
}

type fakeGameConfig struct {
	name      string
	players   []string
	rulesHost string
}

type fakeGameServer struct {
	conn   *net.UDPConn
	config fakeGameConfig
	done   chan struct{}
}

func newFakeGameServer(config fakeGameConfig) (*fakeGameServer, error) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		return nil, err
	}

	server := &fakeGameServer{
		conn:   conn,
		config: config,
		done:   make(chan struct{}),
	}
	go server.serve()
	return server, nil
}

func (s *fakeGameServer) ServerAddr() master.ServerAddr {
	addr := s.conn.LocalAddr().(*net.UDPAddr)
	return master.ServerAddr{
		IP:   append(net.IP(nil), addr.IP.To4()...),
		Port: uint16(addr.Port),
	}
}

func (s *fakeGameServer) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	close(s.done)
	return s.conn.Close()
}

func (s *fakeGameServer) serve() {
	buf := make([]byte, 4096)
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

		response, ok := s.handle(bytes.Clone(buf[:n]))
		if !ok {
			continue
		}
		if _, err := s.conn.WriteToUDP(response, remote); err != nil {
			return
		}
	}
}

func (s *fakeGameServer) handle(req []byte) ([]byte, bool) {
	switch {
	case bytes.Equal(req, buildInfoRequest()):
		return buildInfoResponse(s.config.name), true
	case bytes.Equal(req, buildPlayersRequest(noChallenge)):
		return buildChallengeResponse(0x01020304), true
	case bytes.Equal(req, buildPlayersRequest(0x01020304)):
		return buildPlayersResponse(s.config.players), true
	case bytes.Equal(req, buildRulesRequest(noChallenge)):
		return buildChallengeResponse(0x05060708), true
	case bytes.Equal(req, buildRulesRequest(0x05060708)):
		return buildRulesResponse(s.config.rulesHost), true
	default:
		return nil, false
	}
}

type fakeMasterServer struct {
	conn    *net.UDPConn
	servers []master.ServerAddr
	done    chan struct{}
}

func newFakeMasterServer(servers []master.ServerAddr) (*fakeMasterServer, error) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		return nil, err
	}

	server := &fakeMasterServer{
		conn:    conn,
		servers: append([]master.ServerAddr(nil), servers...),
		done:    make(chan struct{}),
	}
	go server.serve()
	return server, nil
}

func (s *fakeMasterServer) Addr() *net.UDPAddr {
	return s.conn.LocalAddr().(*net.UDPAddr)
}

func (s *fakeMasterServer) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	close(s.done)
	return s.conn.Close()
}

func (s *fakeMasterServer) serve() {
	buf := make([]byte, 2048)
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

		packet := bytes.Clone(buf[:n])
		if len(packet) < 2 || packet[0] != 0x31 {
			continue
		}

		if _, err := s.conn.WriteToUDP(buildMasterResponse(s.servers), remote); err != nil {
			return
		}
	}
}

func buildInfoRequest() []byte {
	return []byte{
		0xFF, 0xFF, 0xFF, 0xFF,
		0x54,
		0x53, 0x6F, 0x75, 0x72, 0x63, 0x65, 0x20, 0x45, 0x6E, 0x67, 0x69, 0x6E, 0x65, 0x20, 0x51, 0x75, 0x65, 0x72, 0x79, 0x00,
	}
}

func buildPlayersRequest(challenge uint32) []byte {
	buf := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x55, 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(buf[5:], challenge)
	return buf
}

func buildRulesRequest(challenge uint32) []byte {
	buf := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x56, 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(buf[5:], challenge)
	return buf
}

func buildChallengeResponse(token uint32) []byte {
	buf := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x41, 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(buf[5:], token)
	return buf
}

func buildInfoResponse(name string) []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x49, 17})
	writeCString(&buf, name)
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
	writeCString(&buf, "scanner,example")
	_ = binary.Write(&buf, binary.LittleEndian, uint64(440))
	return buf.Bytes()
}

func buildPlayersResponse(names []string) []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x44, byte(len(names))})
	for i, name := range names {
		buf.WriteByte(byte(i))
		writeCString(&buf, name)
		_ = binary.Write(&buf, binary.LittleEndian, int32((i+1)*10))
		_ = binary.Write(&buf, binary.LittleEndian, math.Float32bits(float32(i+1)*6.25))
	}
	return buf.Bytes()
}

func buildRulesResponse(hostname string) []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x45})
	_ = binary.Write(&buf, binary.LittleEndian, uint16(2))
	writeCString(&buf, "hostname")
	writeCString(&buf, hostname)
	writeCString(&buf, "version")
	writeCString(&buf, "1.0.0")
	return buf.Bytes()
}

func buildMasterResponse(servers []master.ServerAddr) []byte {
	resp := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x66, 0x0A}
	for _, server := range servers {
		resp = append(resp, server.IP.To4()...)
		var port [2]byte
		binary.BigEndian.PutUint16(port[:], server.Port)
		resp = append(resp, port[:]...)
	}
	return append(resp, 0, 0, 0, 0, 0, 0)
}

func writeCString(buf *bytes.Buffer, value string) {
	buf.WriteString(value)
	buf.WriteByte(0)
}
