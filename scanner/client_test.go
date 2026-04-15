package scanner

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"math"
	"net"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GoFurry/a2s-go/master"
)

const noChallenge = ^uint32(0)

func TestNewClientDefaultsAndValidation(t *testing.T) {
	t.Parallel()

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	if got, want := client.concurrency, defaultConcurrency; got != want {
		t.Fatalf("client.concurrency = %d, want %d", got, want)
	}
	if got, want := client.timeout, defaultTimeout; got != want {
		t.Fatalf("client.timeout = %v, want %v", got, want)
	}
	if got, want := client.maxPacketSize, defaultMaxPacketSize; got != want {
		t.Fatalf("client.maxPacketSize = %d, want %d", got, want)
	}

	if _, err := NewClient(WithConcurrency(0)); err == nil {
		t.Fatal("expected WithConcurrency(0) to fail")
	}
	if _, err := NewClient(WithTimeout(0)); err == nil {
		t.Fatal("expected WithTimeout(0) to fail")
	}
	if _, err := NewClient(WithMaxPacketSize(128)); err == nil {
		t.Fatal("expected WithMaxPacketSize(128) to fail")
	}
}

func TestProbeWithServers(t *testing.T) {
	t.Parallel()

	serverA := newInfoTestServer(t, infoServerBehavior{name: "Alpha"})
	defer serverA.Close()
	serverB := newInfoTestServer(t, infoServerBehavior{name: "Bravo"})
	defer serverB.Close()

	client, err := NewClient(WithConcurrency(2))
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	stream, err := client.Probe(context.Background(), Request{
		Servers: []master.ServerAddr{serverA.ServerAddr(), serverB.ServerAddr()},
	})
	if err != nil {
		t.Fatalf("Probe returned error: %v", err)
	}

	var names []string
	for result := range stream {
		if result.Err != nil {
			t.Fatalf("result.Err = %v, want nil", result.Err)
		}
		names = append(names, result.Info.Name)
	}

	slices.Sort(names)
	if got, want := names, []string{"Alpha", "Bravo"}; !slices.Equal(got, want) {
		t.Fatalf("names = %v, want %v", got, want)
	}
}

func TestCollectReturnsCompletionOrder(t *testing.T) {
	t.Parallel()

	slow := newInfoTestServer(t, infoServerBehavior{name: "Slow", delay: 80 * time.Millisecond})
	defer slow.Close()
	fast := newInfoTestServer(t, infoServerBehavior{name: "Fast"})
	defer fast.Close()

	client, err := NewClient(WithConcurrency(2))
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	results, err := client.Collect(context.Background(), Request{
		Servers: []master.ServerAddr{slow.ServerAddr(), fast.ServerAddr()},
	})
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	if got, want := len(results), 2; got != want {
		t.Fatalf("len(results) = %d, want %d", got, want)
	}
	if got, want := results[0].Info.Name, "Fast"; got != want {
		t.Fatalf("results[0].Info.Name = %q, want %q", got, want)
	}
	if got, want := results[1].Info.Name, "Slow"; got != want {
		t.Fatalf("results[1].Info.Name = %q, want %q", got, want)
	}
}

func TestProbeWithDiscoveryInput(t *testing.T) {
	t.Parallel()

	serverA := newInfoTestServer(t, infoServerBehavior{name: "Disc-A"})
	defer serverA.Close()
	serverB := newInfoTestServer(t, infoServerBehavior{name: "Disc-B"})
	defer serverB.Close()

	discovery := make(chan master.Result, 3)
	discovery <- master.Result{Server: serverA.ServerAddr()}
	discovery <- master.Result{Err: errors.New("master failed")}
	discovery <- master.Result{Server: serverB.ServerAddr()}
	close(discovery)

	client, err := NewClient(WithConcurrency(2))
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	stream, err := client.Probe(context.Background(), Request{Discovery: discovery})
	if err != nil {
		t.Fatalf("Probe returned error: %v", err)
	}

	var names []string
	var discoveryErr error
	for result := range stream {
		if result.Err != nil {
			discoveryErr = result.Err
			continue
		}
		names = append(names, result.Info.Name)
	}

	slices.Sort(names)
	if got, want := names, []string{"Disc-A", "Disc-B"}; !slices.Equal(got, want) {
		t.Fatalf("names = %v, want %v", got, want)
	}
	var scannerErr *Error
	if !errors.As(discoveryErr, &scannerErr) {
		t.Fatalf("expected *Error, got %T", discoveryErr)
	}
	if got, want := scannerErr.Code, ErrorCodeDiscovery; got != want {
		t.Fatalf("scannerErr.Code = %q, want %q", got, want)
	}
}

func TestProbeSingleTargetFailureDoesNotStopOthers(t *testing.T) {
	t.Parallel()

	good := newInfoTestServer(t, infoServerBehavior{name: "Good"})
	defer good.Close()
	bad := newInfoTestServer(t, infoServerBehavior{rawResponse: []byte{0x01, 0x02, 0x03}})
	defer bad.Close()

	client, err := NewClient(WithConcurrency(2))
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	stream, err := client.Probe(context.Background(), Request{
		Servers: []master.ServerAddr{good.ServerAddr(), bad.ServerAddr()},
	})
	if err != nil {
		t.Fatalf("Probe returned error: %v", err)
	}

	var names []string
	var probeErr error
	for result := range stream {
		if result.Err != nil {
			probeErr = result.Err
			continue
		}
		names = append(names, result.Info.Name)
	}

	if got, want := names, []string{"Good"}; !slices.Equal(got, want) {
		t.Fatalf("names = %v, want %v", got, want)
	}
	var scannerErr *Error
	if !errors.As(probeErr, &scannerErr) {
		t.Fatalf("expected *Error, got %T", probeErr)
	}
	if got, want := scannerErr.Code, ErrorCodeProbe; got != want {
		t.Fatalf("scannerErr.Code = %q, want %q", got, want)
	}
}

func TestProbeDoesNotDeduplicate(t *testing.T) {
	t.Parallel()

	server := newInfoTestServer(t, infoServerBehavior{name: "Repeat"})
	defer server.Close()

	client, err := NewClient(WithConcurrency(2))
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	stream, err := client.Probe(context.Background(), Request{
		Servers: []master.ServerAddr{server.ServerAddr(), server.ServerAddr()},
	})
	if err != nil {
		t.Fatalf("Probe returned error: %v", err)
	}

	var count int
	for result := range stream {
		if result.Err != nil {
			t.Fatalf("result.Err = %v, want nil", result.Err)
		}
		count++
	}
	if got, want := count, 2; got != want {
		t.Fatalf("count = %d, want %d", got, want)
	}
	if got, want := server.RequestCount(), 2; got != want {
		t.Fatalf("server.RequestCount() = %d, want %d", got, want)
	}
}

func TestProbeContextCancellationStopsIntake(t *testing.T) {
	t.Parallel()

	blocked := newInfoTestServer(t, infoServerBehavior{name: "Blocked", gate: make(chan struct{})})
	defer blocked.Close()
	skipped := newInfoTestServer(t, infoServerBehavior{name: "Skipped"})
	defer skipped.Close()

	client, err := NewClient(WithConcurrency(1))
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	stream, err := client.Probe(ctx, Request{
		Servers: []master.ServerAddr{blocked.ServerAddr(), skipped.ServerAddr()},
	})
	if err != nil {
		t.Fatalf("Probe returned error: %v", err)
	}

	deadline := time.Now().Add(300 * time.Millisecond)
	for blocked.RequestCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if blocked.RequestCount() == 0 {
		t.Fatal("expected first probe to start before canceling")
	}

	cancel()
	close(blocked.behavior.gate)

	var results []Result
	for result := range stream {
		results = append(results, result)
	}

	if got, want := len(results), 1; got != want {
		t.Fatalf("len(results) = %d, want %d", got, want)
	}
	if got, want := skipped.RequestCount(), 0; got != want {
		t.Fatalf("skipped.RequestCount() = %d, want %d", got, want)
	}
}

func TestProbeRespectsConcurrencyLimit(t *testing.T) {
	t.Parallel()

	gate := make(chan struct{})
	var current int32
	var maxSeen int32

	var servers []*infoTestServer
	var inputs []master.ServerAddr
	for i := 0; i < 5; i++ {
		server := newInfoTestServer(t, infoServerBehavior{
			name: "Concurrent",
			onRequest: func() {
				now := atomic.AddInt32(&current, 1)
				for {
					seen := atomic.LoadInt32(&maxSeen)
					if now <= seen || atomic.CompareAndSwapInt32(&maxSeen, seen, now) {
						break
					}
				}
			},
			onResponseDone: func() {
				atomic.AddInt32(&current, -1)
			},
			gate: gate,
		})
		defer server.Close()
		servers = append(servers, server)
		inputs = append(inputs, server.ServerAddr())
	}

	client, err := NewClient(WithConcurrency(2))
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	done := make(chan []Result, 1)
	go func() {
		results, _ := client.Collect(context.Background(), Request{Servers: inputs})
		done <- results
	}()

	time.Sleep(80 * time.Millisecond)
	close(gate)

	results := <-done
	if got, want := len(results), 5; got != want {
		t.Fatalf("len(results) = %d, want %d", got, want)
	}
	if got, want := atomic.LoadInt32(&maxSeen), int32(2); got != want {
		t.Fatalf("max concurrency = %d, want %d", got, want)
	}
}

func TestProbeRejectsInvalidInputShape(t *testing.T) {
	t.Parallel()

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	discovery := make(chan master.Result)
	defer close(discovery)

	if _, err := client.Probe(context.Background(), Request{}); err == nil {
		t.Fatal("expected Probe to reject missing inputs")
	}
	if _, err := client.Probe(context.Background(), Request{
		Servers:   []master.ServerAddr{},
		Discovery: discovery,
	}); err == nil {
		t.Fatal("expected Probe to reject multiple inputs")
	}
}

func TestProbePlayersWithServers(t *testing.T) {
	t.Parallel()

	serverA := newGameTestServer(t, gameServerBehavior{
		infoName:    "Players-A",
		playerNames: []string{"alice", "bob"},
	})
	defer serverA.Close()

	serverB := newGameTestServer(t, gameServerBehavior{
		infoName:    "Players-B",
		playerNames: []string{"carol"},
	})
	defer serverB.Close()

	client, err := NewClient(WithConcurrency(2))
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	stream, err := client.ProbePlayers(context.Background(), Request{
		Servers: []master.ServerAddr{serverA.ServerAddr(), serverB.ServerAddr()},
	})
	if err != nil {
		t.Fatalf("ProbePlayers returned error: %v", err)
	}

	counts := map[string]uint8{}
	for result := range stream {
		if result.Err != nil {
			t.Fatalf("result.Err = %v, want nil", result.Err)
		}
		counts[result.Server.String()] = result.Players.Count
	}

	if got, want := counts[serverA.ServerAddr().String()], uint8(2); got != want {
		t.Fatalf("serverA players.Count = %d, want %d", got, want)
	}
	if got, want := counts[serverB.ServerAddr().String()], uint8(1); got != want {
		t.Fatalf("serverB players.Count = %d, want %d", got, want)
	}
}

func TestCollectRulesReturnsCompletionOrder(t *testing.T) {
	t.Parallel()

	slow := newGameTestServer(t, gameServerBehavior{
		infoName:   "Rules-Slow",
		rulesValue: "slow-host",
		delay:      80 * time.Millisecond,
	})
	defer slow.Close()

	fast := newGameTestServer(t, gameServerBehavior{
		infoName:   "Rules-Fast",
		rulesValue: "fast-host",
	})
	defer fast.Close()

	client, err := NewClient(WithConcurrency(2))
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	results, err := client.CollectRules(context.Background(), Request{
		Servers: []master.ServerAddr{slow.ServerAddr(), fast.ServerAddr()},
	})
	if err != nil {
		t.Fatalf("CollectRules returned error: %v", err)
	}
	if got, want := len(results), 2; got != want {
		t.Fatalf("len(results) = %d, want %d", got, want)
	}
	if got, want := results[0].Rules.Items["hostname"], "fast-host"; got != want {
		t.Fatalf("results[0].Rules.Items[hostname] = %q, want %q", got, want)
	}
	if got, want := results[1].Rules.Items["hostname"], "slow-host"; got != want {
		t.Fatalf("results[1].Rules.Items[hostname] = %q, want %q", got, want)
	}
}

func TestProbeRulesWithDiscoveryInput(t *testing.T) {
	t.Parallel()

	serverA := newGameTestServer(t, gameServerBehavior{
		infoName:   "Rules-Disc-A",
		rulesValue: "disc-a",
	})
	defer serverA.Close()

	serverB := newGameTestServer(t, gameServerBehavior{
		infoName:   "Rules-Disc-B",
		rulesValue: "disc-b",
	})
	defer serverB.Close()

	discovery := make(chan master.Result, 3)
	discovery <- master.Result{Server: serverA.ServerAddr()}
	discovery <- master.Result{Err: errors.New("master failed")}
	discovery <- master.Result{Server: serverB.ServerAddr()}
	close(discovery)

	client, err := NewClient(WithConcurrency(2))
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	stream, err := client.ProbeRules(context.Background(), Request{Discovery: discovery})
	if err != nil {
		t.Fatalf("ProbeRules returned error: %v", err)
	}

	var hostnames []string
	var discoveryErr error
	for result := range stream {
		if result.Err != nil {
			discoveryErr = result.Err
			continue
		}
		hostnames = append(hostnames, result.Rules.Items["hostname"])
	}

	slices.Sort(hostnames)
	if got, want := hostnames, []string{"disc-a", "disc-b"}; !slices.Equal(got, want) {
		t.Fatalf("hostnames = %v, want %v", got, want)
	}

	var scannerErr *Error
	if !errors.As(discoveryErr, &scannerErr) {
		t.Fatalf("expected *Error, got %T", discoveryErr)
	}
	if got, want := scannerErr.Code, ErrorCodeDiscovery; got != want {
		t.Fatalf("scannerErr.Code = %q, want %q", got, want)
	}
}

type infoServerBehavior struct {
	name           string
	delay          time.Duration
	rawResponse    []byte
	gate           chan struct{}
	onRequest      func()
	onResponseDone func()
}

type infoTestServer struct {
	t        *testing.T
	conn     *net.UDPConn
	behavior infoServerBehavior
	done     chan struct{}
	requests atomic.Int32
}

func newInfoTestServer(t *testing.T, behavior infoServerBehavior) *infoTestServer {
	t.Helper()

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("ListenUDP returned error: %v", err)
	}
	server := &infoTestServer{
		t:        t,
		conn:     conn,
		behavior: behavior,
		done:     make(chan struct{}),
	}
	go server.serve()
	return server
}

func (s *infoTestServer) ServerAddr() master.ServerAddr {
	addr := s.conn.LocalAddr().(*net.UDPAddr)
	return master.ServerAddr{
		IP:   append(net.IP(nil), addr.IP.To4()...),
		Port: uint16(addr.Port),
	}
}

func (s *infoTestServer) RequestCount() int {
	return int(s.requests.Load())
}

func (s *infoTestServer) Close() {
	close(s.done)
	_ = s.conn.Close()
}

func (s *infoTestServer) serve() {
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

		s.requests.Add(1)
		if s.behavior.onRequest != nil {
			s.behavior.onRequest()
		}
		if s.behavior.gate != nil {
			<-s.behavior.gate
		}
		if s.behavior.delay > 0 {
			time.Sleep(s.behavior.delay)
		}

		response := s.behavior.rawResponse
		if response == nil {
			response = buildInfoResponse(s.behavior.name)
		}
		if _, err := s.conn.WriteToUDP(response, remote); err != nil {
			select {
			case <-s.done:
				return
			default:
				s.t.Errorf("WriteToUDP returned error: %v", err)
				return
			}
		}
		if s.behavior.onResponseDone != nil {
			s.behavior.onResponseDone()
		}
		_ = n
	}
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
	writeCString(&buf, "payload,test")
	_ = binary.Write(&buf, binary.LittleEndian, uint64(440))
	return buf.Bytes()
}

func buildInfoRequestBytes() []byte {
	return []byte{
		0xFF, 0xFF, 0xFF, 0xFF,
		0x54,
		0x53, 0x6F, 0x75, 0x72, 0x63, 0x65, 0x20, 0x45, 0x6E, 0x67, 0x69, 0x6E, 0x65, 0x20, 0x51, 0x75, 0x65, 0x72, 0x79, 0x00,
	}
}

func writeCString(buf *bytes.Buffer, value string) {
	buf.WriteString(value)
	buf.WriteByte(0)
}

type gameServerBehavior struct {
	infoName    string
	playerNames []string
	rulesValue  string
	delay       time.Duration
}

type gameTestServer struct {
	t        *testing.T
	conn     *net.UDPConn
	behavior gameServerBehavior
	done     chan struct{}
}

func newGameTestServer(t *testing.T, behavior gameServerBehavior) *gameTestServer {
	t.Helper()

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("ListenUDP returned error: %v", err)
	}

	server := &gameTestServer{
		t:        t,
		conn:     conn,
		behavior: behavior,
		done:     make(chan struct{}),
	}
	go server.serve()
	return server
}

func (s *gameTestServer) ServerAddr() master.ServerAddr {
	addr := s.conn.LocalAddr().(*net.UDPAddr)
	return master.ServerAddr{
		IP:   append(net.IP(nil), addr.IP.To4()...),
		Port: uint16(addr.Port),
	}
}

func (s *gameTestServer) Close() {
	close(s.done)
	_ = s.conn.Close()
}

func (s *gameTestServer) serve() {
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
		if s.behavior.delay > 0 {
			time.Sleep(s.behavior.delay)
		}

		response, ok := s.handle(req)
		if !ok {
			s.t.Errorf("unexpected request: %v", req)
			return
		}
		if _, err := s.conn.WriteToUDP(response, remote); err != nil {
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

func (s *gameTestServer) handle(req []byte) ([]byte, bool) {
	switch {
	case bytes.Equal(req, buildInfoRequestBytes()):
		return buildInfoResponse(s.behavior.infoName), true
	case bytes.Equal(req, buildPlayersRequestBytes(noChallenge)):
		return buildChallengeResponse(0x01020304), true
	case bytes.Equal(req, buildPlayersRequestBytes(0x01020304)):
		return buildPlayersResponse(s.behavior.playerNames), true
	case bytes.Equal(req, buildRulesRequestBytes(noChallenge)):
		return buildChallengeResponse(0x05060708), true
	case bytes.Equal(req, buildRulesRequestBytes(0x05060708)):
		return buildRulesResponse(s.behavior.rulesValue), true
	default:
		return nil, false
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

func buildPlayersResponse(names []string) []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x44, byte(len(names))})
	for i, name := range names {
		buf.WriteByte(byte(i))
		writeCString(&buf, name)
		_ = binary.Write(&buf, binary.LittleEndian, int32((i+1)*10))
		_ = binary.Write(&buf, binary.LittleEndian, math.Float32bits(float32(i+1)*7.5))
	}
	return buf.Bytes()
}

func buildRulesResponse(hostname string) []byte {
	if hostname == "" {
		hostname = "scanner-rules"
	}
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x45})
	_ = binary.Write(&buf, binary.LittleEndian, uint16(2))
	writeCString(&buf, "hostname")
	writeCString(&buf, hostname)
	writeCString(&buf, "version")
	writeCString(&buf, "1.0.0")
	return buf.Bytes()
}
