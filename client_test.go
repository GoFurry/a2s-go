package a2s

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"math"
	"net"
	"slices"
	"sync"
	"testing"
	"time"
)

const noChallenge = ^uint32(0)

func TestNewClientDefaultsPort(t *testing.T) {
	t.Parallel()

	addr, err := normalizeAddress("127.0.0.1")
	if err != nil {
		t.Fatalf("normalizeAddress returned error: %v", err)
	}
	if got, want := addr.Port, defaultPort; got != want {
		t.Fatalf("addr.Port = %d, want %d", got, want)
	}
}

func TestOptionValidation(t *testing.T) {
	t.Parallel()

	if _, err := NewClient("127.0.0.1", WithTimeout(0)); err == nil {
		t.Fatal("expected WithTimeout(0) to fail")
	}
	if _, err := NewClient("127.0.0.1", WithMaxPacketSize(128)); err == nil {
		t.Fatal("expected WithMaxPacketSize(128) to fail")
	}
}

func TestQueryInfo(t *testing.T) {
	t.Parallel()

	server := newUDPTestServer(t, func(state *udpTestState, req []byte) [][]byte {
		if !bytes.Equal(req, buildInfoRequestBytes()) {
			t.Fatalf("unexpected request: %v", req)
		}
		return [][]byte{buildInfoResponse()}
	})
	defer server.Close()

	client, err := NewClient(server.Addr().String())
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	defer client.Close()

	info, err := client.QueryInfo(context.Background())
	if err != nil {
		t.Fatalf("QueryInfo returned error: %v", err)
	}
	if info.Name != "Codex Test Server" {
		t.Fatalf("info.Name = %q, want %q", info.Name, "Codex Test Server")
	}
	if info.Map != "cp_badlands" {
		t.Fatalf("info.Map = %q, want %q", info.Map, "cp_badlands")
	}
	if info.Port != 27015 {
		t.Fatalf("info.Port = %d, want 27015", info.Port)
	}
	if info.SteamID != 76561198000000000 {
		t.Fatalf("info.SteamID = %d, want 76561198000000000", info.SteamID)
	}
	if info.ServerType != ServerTypeDedicated {
		t.Fatalf("info.ServerType = %q, want %q", info.ServerType.String(), ServerTypeDedicated.String())
	}
	if info.Environment != EnvironmentLinux {
		t.Fatalf("info.Environment = %q, want %q", info.Environment.String(), EnvironmentLinux.String())
	}
}

func TestQueryInfoWithChallengeFallback(t *testing.T) {
	t.Parallel()

	var attempts int
	server := newUDPTestServer(t, func(state *udpTestState, req []byte) [][]byte {
		attempts++
		switch attempts {
		case 1:
			if !bytes.Equal(req, buildInfoRequestBytes()) {
				t.Fatalf("unexpected first request: %v", req)
			}
			return [][]byte{buildChallengeResponse(0x01020304)}
		case 2:
			expected := append(buildInfoRequestBytes(), []byte{0x04, 0x03, 0x02, 0x01}...)
			if !bytes.Equal(req, expected) {
				t.Fatalf("unexpected second request: %v", req)
			}
			return [][]byte{buildInfoResponse()}
		default:
			t.Fatalf("unexpected request count: %d", attempts)
			return nil
		}
	})
	defer server.Close()

	client, err := NewClient(server.Addr().String())
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	defer client.Close()

	info, err := client.QueryInfo(context.Background())
	if err != nil {
		t.Fatalf("QueryInfo returned error: %v", err)
	}
	if info.Name != "Codex Test Server" {
		t.Fatalf("info.Name = %q, want %q", info.Name, "Codex Test Server")
	}
}

func TestQueryPlayersWithChallenge(t *testing.T) {
	t.Parallel()

	var attempts int
	server := newUDPTestServer(t, func(state *udpTestState, req []byte) [][]byte {
		attempts++
		switch attempts {
		case 1:
			if !bytes.Equal(req, buildPlayersRequestBytes(noChallenge)) {
				t.Fatalf("unexpected first request: %v", req)
			}
			return [][]byte{buildChallengeResponse(0x0A0B0C0D)}
		case 2:
			expected := buildPlayersRequestBytes(0x0A0B0C0D)
			if !bytes.Equal(req, expected) {
				t.Fatalf("unexpected second request: %v", req)
			}
			return [][]byte{buildPlayersResponse()}
		default:
			t.Fatalf("unexpected request count: %d", attempts)
			return nil
		}
	})
	defer server.Close()

	client, err := NewClient(server.Addr().String())
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	defer client.Close()

	players, err := client.QueryPlayers(context.Background())
	if err != nil {
		t.Fatalf("QueryPlayers returned error: %v", err)
	}
	if players.Count != 2 {
		t.Fatalf("players.Count = %d, want 2", players.Count)
	}
	if len(players.Players) != 2 {
		t.Fatalf("len(players.Players) = %d, want 2", len(players.Players))
	}
	if players.Players[1].Name != "player-two" {
		t.Fatalf("players.Players[1].Name = %q, want %q", players.Players[1].Name, "player-two")
	}
}

func TestQueryInfoTheShip(t *testing.T) {
	t.Parallel()

	server := newUDPTestServer(t, func(state *udpTestState, req []byte) [][]byte {
		if !bytes.Equal(req, buildInfoRequestBytes()) {
			t.Fatalf("unexpected request: %v", req)
		}
		return [][]byte{buildInfoResponseTheShip()}
	})
	defer server.Close()

	client, err := NewClient(server.Addr().String())
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	defer client.Close()

	info, err := client.QueryInfo(context.Background())
	if err != nil {
		t.Fatalf("QueryInfo returned error: %v", err)
	}
	if info.TheShip == nil {
		t.Fatal("expected TheShip metadata")
	}
	if info.TheShip.Mode != TheShipModeDeathmatch {
		t.Fatalf("info.TheShip.Mode = %q, want %q", info.TheShip.Mode.String(), TheShipModeDeathmatch.String())
	}
	if info.TheShip.Witnesses != 2 {
		t.Fatalf("info.TheShip.Witnesses = %d, want 2", info.TheShip.Witnesses)
	}
	if info.TheShip.Duration != 15 {
		t.Fatalf("info.TheShip.Duration = %d, want 15", info.TheShip.Duration)
	}
}

func TestQueryPlayersTheShip(t *testing.T) {
	t.Parallel()

	server := newUDPTestServer(t, func(state *udpTestState, req []byte) [][]byte {
		if !bytes.Equal(req, buildPlayersRequestBytes(noChallenge)) {
			t.Fatalf("unexpected request: %v", req)
		}
		return [][]byte{buildPlayersResponseTheShip()}
	})
	defer server.Close()

	client, err := NewClient(server.Addr().String())
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	defer client.Close()

	players, err := client.QueryPlayers(context.Background())
	if err != nil {
		t.Fatalf("QueryPlayers returned error: %v", err)
	}
	if players.Players[0].TheShip == nil {
		t.Fatal("expected The Ship player metadata")
	}
	if players.Players[0].TheShip.Deaths != 3 {
		t.Fatalf("players.Players[0].TheShip.Deaths = %d, want 3", players.Players[0].TheShip.Deaths)
	}
	if players.Players[1].TheShip.Money != 900 {
		t.Fatalf("players.Players[1].TheShip.Money = %d, want 900", players.Players[1].TheShip.Money)
	}
}

func TestQueryRulesWithSplitPackets(t *testing.T) {
	t.Parallel()

	payload := buildRulesPayload()
	parts := buildSplitRulesResponses(payload, false)
	server := newUDPTestServer(t, func(state *udpTestState, req []byte) [][]byte {
		if !bytes.Equal(req, buildRulesRequestBytes(noChallenge)) {
			t.Fatalf("unexpected request: %v", req)
		}
		return parts
	})
	defer server.Close()

	client, err := NewClient(server.Addr().String())
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	defer client.Close()

	rules, err := client.QueryRules(context.Background())
	if err != nil {
		t.Fatalf("QueryRules returned error: %v", err)
	}
	if rules.Count != 3 {
		t.Fatalf("rules.Count = %d, want 3", rules.Count)
	}
	if rules.ReportedCount != 3 {
		t.Fatalf("rules.ReportedCount = %d, want 3", rules.ReportedCount)
	}
	if rules.Truncated {
		t.Fatal("rules.Truncated = true, want false")
	}
	if rules.Items["hostname"] != "Codex Rules Server" {
		t.Fatalf("rules.Items[hostname] = %q, want %q", rules.Items["hostname"], "Codex Rules Server")
	}
}

func TestQueryRulesWithCompressedSplitPackets(t *testing.T) {
	t.Parallel()

	payload := buildRulesPayload()
	parts := buildSplitRulesResponses(buildCompressedRulesPayload(payload), true)
	server := newUDPTestServer(t, func(state *udpTestState, req []byte) [][]byte {
		if !bytes.Equal(req, buildRulesRequestBytes(noChallenge)) {
			t.Fatalf("unexpected request: %v", req)
		}
		return parts
	})
	defer server.Close()

	client, err := NewClient(server.Addr().String())
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	defer client.Close()

	rules, err := client.QueryRules(context.Background())
	if err != nil {
		t.Fatalf("QueryRules returned error: %v", err)
	}
	if got, want := rules.Items["sv_tags"], "payload,test"; got != want {
		t.Fatalf("rules.Items[sv_tags] = %q, want %q", got, want)
	}
}

func TestParseRulesTruncated(t *testing.T) {
	t.Parallel()

	rules, err := parseRules(buildTruncatedRulesPayload())
	if err != nil {
		t.Fatalf("parseRules returned error: %v", err)
	}
	if rules.ReportedCount != 3 {
		t.Fatalf("rules.ReportedCount = %d, want 3", rules.ReportedCount)
	}
	if rules.Count != 2 {
		t.Fatalf("rules.Count = %d, want 2", rules.Count)
	}
	if !rules.Truncated {
		t.Fatal("rules.Truncated = false, want true")
	}
	if rules.Items["hostname"] != "Codex Rules Server" {
		t.Fatalf("rules.Items[hostname] = %q, want %q", rules.Items["hostname"], "Codex Rules Server")
	}
}

func TestContextTimeout(t *testing.T) {
	t.Parallel()

	server := newUDPTestServer(t, func(state *udpTestState, req []byte) [][]byte {
		time.Sleep(120 * time.Millisecond)
		return [][]byte{buildInfoResponse()}
	})
	defer server.Close()

	client, err := NewClient(server.Addr().String(), WithTimeout(50*time.Millisecond))
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = client.QueryInfo(ctx)
	if err == nil {
		t.Fatal("expected QueryInfo to time out")
	}
	var a2sErr *Error
	if !errors.As(err, &a2sErr) {
		t.Fatalf("expected *Error, got %T", err)
	}
	if a2sErr.Code != ErrorCodeTimeout {
		t.Fatalf("error.Code = %q, want %q", a2sErr.Code, ErrorCodeTimeout)
	}
}

type udpTestState struct {
	mu       sync.Mutex
	requests [][]byte
}

type udpTestServer struct {
	t       *testing.T
	conn    *net.UDPConn
	handler func(*udpTestState, []byte) [][]byte
	done    chan struct{}
	state   udpTestState
}

func newUDPTestServer(t *testing.T, handler func(*udpTestState, []byte) [][]byte) *udpTestServer {
	t.Helper()

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("ListenUDP returned error: %v", err)
	}
	server := &udpTestServer{
		t:       t,
		conn:    conn,
		handler: handler,
		done:    make(chan struct{}),
	}
	go server.serve()
	return server
}

func (s *udpTestServer) Addr() *net.UDPAddr {
	return s.conn.LocalAddr().(*net.UDPAddr)
}

func (s *udpTestServer) Close() {
	close(s.done)
	_ = s.conn.Close()
}

func (s *udpTestServer) serve() {
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
			select {
			case <-s.done:
				return
			default:
				s.t.Errorf("ReadFromUDP returned error: %v", err)
				return
			}
		}

		req := slices.Clone(buf[:n])
		s.state.mu.Lock()
		s.state.requests = append(s.state.requests, req)
		s.state.mu.Unlock()

		for _, resp := range s.handler(&s.state, req) {
			if _, err := s.conn.WriteToUDP(resp, remote); err != nil {
				select {
				case <-s.done:
					return
				default:
					s.t.Errorf("WriteToUDP returned error: %v", err)
					return
				}
			}
		}
	}
}

func buildInfoRequestBytes() []byte {
	return []byte{
		0xFF, 0xFF, 0xFF, 0xFF,
		0x54,
		0x53, 0x6F, 0x75, 0x72, 0x63, 0x65, 0x20, 0x45, 0x6E, 0x67, 0x69, 0x6E, 0x65, 0x20, 0x51, 0x75, 0x65, 0x72, 0x79, 0x00,
	}
}

func buildPlayersRequestBytes(challenge uint32) []byte {
	buf := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x55, 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(buf[5:], challenge)
	return buf
}

func buildRulesRequestBytes(challenge uint32) []byte {
	buf := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x56, 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(buf[5:], challenge)
	return buf
}

func buildChallengeResponse(token uint32) []byte {
	buf := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x41, 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(buf[5:], token)
	return buf
}

func buildInfoResponse() []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x49, 17})
	writeCString(&buf, "Codex Test Server")
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
	writeCString(&buf, "payload,test")
	_ = binary.Write(&buf, binary.LittleEndian, uint64(440))
	return buf.Bytes()
}

func buildInfoResponseTheShip() []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x49, 17})
	writeCString(&buf, "The Ship Test Server")
	writeCString(&buf, "ts_test")
	writeCString(&buf, "ship")
	writeCString(&buf, "The Ship")
	_ = binary.Write(&buf, binary.LittleEndian, uint16(2400))
	buf.WriteByte(8)
	buf.WriteByte(16)
	buf.WriteByte(0)
	buf.WriteByte('d')
	buf.WriteByte('w')
	buf.WriteByte(0)
	buf.WriteByte(1)
	buf.WriteByte(byte(TheShipModeDeathmatch))
	buf.WriteByte(2)
	buf.WriteByte(15)
	writeCString(&buf, "1.2.3")
	return buf.Bytes()
}

func buildPlayersResponse() []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x44, 0x02})
	buf.WriteByte(0)
	writeCString(&buf, "player-one")
	_ = binary.Write(&buf, binary.LittleEndian, int32(10))
	_ = binary.Write(&buf, binary.LittleEndian, math.Float32bits(13.5))
	buf.WriteByte(1)
	writeCString(&buf, "player-two")
	_ = binary.Write(&buf, binary.LittleEndian, int32(25))
	_ = binary.Write(&buf, binary.LittleEndian, math.Float32bits(22.75))
	return buf.Bytes()
}

func buildPlayersResponseTheShip() []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x44, 0x02})
	buf.WriteByte(0)
	writeCString(&buf, "shipmate-one")
	_ = binary.Write(&buf, binary.LittleEndian, int32(12))
	_ = binary.Write(&buf, binary.LittleEndian, math.Float32bits(33.5))
	buf.WriteByte(1)
	writeCString(&buf, "shipmate-two")
	_ = binary.Write(&buf, binary.LittleEndian, int32(19))
	_ = binary.Write(&buf, binary.LittleEndian, math.Float32bits(41.25))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(3))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(500))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(7))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(900))
	return buf.Bytes()
}

func buildRulesPayload() []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x45})
	_ = binary.Write(&buf, binary.LittleEndian, uint16(3))
	writeCString(&buf, "hostname")
	writeCString(&buf, "Codex Rules Server")
	writeCString(&buf, "sv_tags")
	writeCString(&buf, "payload,test")
	writeCString(&buf, "version")
	writeCString(&buf, "1.0.0")
	return buf.Bytes()
}

func buildTruncatedRulesPayload() []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x45})
	_ = binary.Write(&buf, binary.LittleEndian, uint16(3))
	writeCString(&buf, "hostname")
	writeCString(&buf, "Codex Rules Server")
	writeCString(&buf, "sv_tags")
	writeCString(&buf, "payload,test")
	writeCString(&buf, "version")
	buf.WriteString("1.0")
	return buf.Bytes()
}

func buildSplitRulesResponses(payload []byte, compressed bool) [][]byte {
	id := uint32(0x11223344)
	if compressed {
		id |= 0x80000000
	}
	chunks := splitPayload(payload, 2)
	out := make([][]byte, 0, len(chunks))
	for i, chunk := range chunks {
		var buf bytes.Buffer
		_ = binary.Write(&buf, binary.LittleEndian, int32(-2))
		_ = binary.Write(&buf, binary.LittleEndian, id)
		buf.WriteByte(byte(len(chunks)))
		buf.WriteByte(byte(i))
		_ = binary.Write(&buf, binary.LittleEndian, uint16(len(payload)))
		buf.Write(chunk)
		out = append(out, buf.Bytes())
	}
	return out
}

func buildCompressedRulesPayload(payload []byte) []byte {
	compressed := mustBzip2Compress()
	out := make([]byte, 8+len(compressed))
	binary.LittleEndian.PutUint32(out[:4], uint32(len(payload)))
	binary.LittleEndian.PutUint32(out[4:8], crc32.ChecksumIEEE(payload))
	copy(out[8:], compressed)
	return out
}

func splitPayload(payload []byte, parts int) [][]byte {
	step := (len(payload) + parts - 1) / parts
	out := make([][]byte, 0, parts)
	for start := 0; start < len(payload); start += step {
		end := start + step
		if end > len(payload) {
			end = len(payload)
		}
		out = append(out, slices.Clone(payload[start:end]))
	}
	return out
}

func writeCString(buf *bytes.Buffer, value string) {
	buf.WriteString(value)
	buf.WriteByte(0)
}

func mustBzip2Compress() []byte {
	return []byte{
		0x42, 0x5a, 0x68, 0x39, 0x31, 0x41, 0x59, 0x26, 0x53, 0x59, 0xbd, 0x58,
		0x86, 0x5c, 0x00, 0x00, 0x23, 0x5f, 0x80, 0xc8, 0x00, 0x40, 0x05, 0x60,
		0x00, 0x0a, 0x00, 0x18, 0x00, 0xa6, 0xe7, 0xdf, 0x60, 0x00, 0x00, 0xa0,
		0x00, 0x48, 0xa4, 0x3d, 0x46, 0x47, 0xa8, 0xf2, 0x83, 0x46, 0x9e, 0x93,
		0xc5, 0x0c, 0x7a, 0x13, 0x04, 0xc0, 0x04, 0x60, 0xd2, 0x86, 0xe0, 0x89,
		0x61, 0x26, 0xc6, 0x80, 0x67, 0x0d, 0xa2, 0x50, 0x6f, 0x87, 0x75, 0xa8,
		0x2e, 0x50, 0xaa, 0xcc, 0xa4, 0x26, 0xce, 0x03, 0x29, 0xa3, 0xbe, 0x96,
		0x8b, 0x85, 0xc1, 0x46, 0x68, 0xec, 0x82, 0x08, 0xf1, 0x0e, 0x1e, 0xcb,
		0xc1, 0x77, 0x24, 0x53, 0x85, 0x09, 0x0b, 0xd5, 0x88, 0x65, 0xc0,
	}
}
