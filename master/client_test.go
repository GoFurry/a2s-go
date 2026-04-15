package master

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestDefaultConfigAndOptionValidation(t *testing.T) {
	t.Parallel()

	cfg := defaultClientConfig()
	if got, want := cfg.baseAddress, defaultBaseAddress; got != want {
		t.Fatalf("cfg.baseAddress = %q, want %q", got, want)
	}
	if got, want := cfg.timeout, defaultTimeout; got != want {
		t.Fatalf("cfg.timeout = %v, want %v", got, want)
	}

	if _, err := NewClient(WithTimeout(0)); err == nil {
		t.Fatal("expected WithTimeout(0) to fail")
	}
	if _, err := NewClient(WithMaxPacketSize(32)); err == nil {
		t.Fatal("expected WithMaxPacketSize(32) to fail")
	}
	if _, err := NewClient(WithBaseAddress("")); err == nil {
		t.Fatal("expected empty base address to fail")
	}
}

func TestQueryUsesStartCursorAndReturnsNextCursor(t *testing.T) {
	t.Parallel()

	server := newMasterTestServer(t, func(state *masterTestState, req []byte) [][]byte {
		state.recordRequest(req)

		parsed := parseMasterRequest(t, req)
		if got, want := parsed.Region, byte(RegionAsia); got != want {
			t.Fatalf("parsed.Region = %d, want %d", got, want)
		}
		if got, want := parsed.Cursor, "0.0.0.0:0"; got != want {
			t.Fatalf("parsed.Cursor = %q, want %q", got, want)
		}
		if got, want := parsed.Filter, "\\gamedir\\tf"; got != want {
			t.Fatalf("parsed.Filter = %q, want %q", got, want)
		}

		return [][]byte{buildMasterResponse(
			masterAddr("1.2.3.4", 27015),
			masterAddr("5.6.7.8", 27016),
		)}
	})
	defer server.Close()

	client, err := NewClient(WithBaseAddress(server.Addr().String()))
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	defer client.Close()

	page, err := client.Query(context.Background(), Request{
		Region: RegionAsia,
		Filter: "\\gamedir\\tf",
	})
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if page.Done {
		t.Fatal("page.Done = true, want false")
	}
	if got, want := len(page.Servers), 2; got != want {
		t.Fatalf("len(page.Servers) = %d, want %d", got, want)
	}
	if got, want := page.Servers[0].String(), "1.2.3.4:27015"; got != want {
		t.Fatalf("page.Servers[0] = %q, want %q", got, want)
	}
	if got, want := page.NextCursor.String(), "5.6.7.8:27016"; got != want {
		t.Fatalf("page.NextCursor = %q, want %q", got, want)
	}
	if page.NextCursor.IsTerminal() {
		t.Fatal("page.NextCursor.IsTerminal() = true, want false")
	}
}

func TestQueryTerminalPageDropsSentinel(t *testing.T) {
	t.Parallel()

	server := newMasterTestServer(t, func(state *masterTestState, req []byte) [][]byte {
		return [][]byte{buildMasterResponse(
			masterAddr("9.9.9.9", 27015),
			masterAddr("0.0.0.0", 0),
		)}
	})
	defer server.Close()

	client, err := NewClient(WithBaseAddress(server.Addr().String()))
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	defer client.Close()

	page, err := client.Query(context.Background(), Request{Region: RegionEurope})
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if !page.Done {
		t.Fatal("page.Done = false, want true")
	}
	if got, want := len(page.Servers), 1; got != want {
		t.Fatalf("len(page.Servers) = %d, want %d", got, want)
	}
	if got, want := page.Servers[0].String(), "9.9.9.9:27015"; got != want {
		t.Fatalf("page.Servers[0] = %q, want %q", got, want)
	}
	if !page.NextCursor.IsTerminal() {
		t.Fatal("page.NextCursor.IsTerminal() = false, want true")
	}
	if page.NextCursor.IsZero() {
		t.Fatal("page.NextCursor.IsZero() = true, want false")
	}
	if got, want := page.NextCursor.String(), "0.0.0.0:0"; got != want {
		t.Fatalf("page.NextCursor.String() = %q, want %q", got, want)
	}
}

func TestQueryEmptyTerminalPage(t *testing.T) {
	t.Parallel()

	server := newMasterTestServer(t, func(state *masterTestState, req []byte) [][]byte {
		return [][]byte{buildMasterResponse(masterAddr("0.0.0.0", 0))}
	})
	defer server.Close()

	client, err := NewClient(WithBaseAddress(server.Addr().String()))
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	defer client.Close()

	page, err := client.Query(context.Background(), Request{Region: RegionRestOfWorld})
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if !page.Done {
		t.Fatal("page.Done = false, want true")
	}
	if len(page.Servers) != 0 {
		t.Fatalf("len(page.Servers) = %d, want 0", len(page.Servers))
	}
	if !page.NextCursor.IsTerminal() {
		t.Fatal("page.NextCursor.IsTerminal() = false, want true")
	}
}

func TestStreamAcrossPages(t *testing.T) {
	t.Parallel()

	server := newMasterTestServer(t, func(state *masterTestState, req []byte) [][]byte {
		state.recordRequest(req)
		switch parseMasterRequest(t, req).Cursor {
		case "0.0.0.0:0":
			return [][]byte{buildMasterResponse(
				masterAddr("1.1.1.1", 27015),
				masterAddr("2.2.2.2", 27016),
			)}
		case "2.2.2.2:27016":
			return [][]byte{buildMasterResponse(
				masterAddr("3.3.3.3", 27017),
				masterAddr("0.0.0.0", 0),
			)}
		default:
			t.Fatalf("unexpected cursor: %q", parseMasterRequest(t, req).Cursor)
			return nil
		}
	})
	defer server.Close()

	client, err := NewClient(WithBaseAddress(server.Addr().String()))
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	defer client.Close()

	stream, err := client.Stream(context.Background(), Request{
		Region: RegionUSEast,
		Filter: "\\secure\\1",
	})
	if err != nil {
		t.Fatalf("Stream returned error: %v", err)
	}

	var got []string
	for result := range stream {
		if result.Err != nil {
			t.Fatalf("result.Err = %v, want nil", result.Err)
		}
		got = append(got, result.Server.String())
	}

	want := []string{"1.1.1.1:27015", "2.2.2.2:27016", "3.3.3.3:27017"}
	if !slices.Equal(got, want) {
		t.Fatalf("stream results = %v, want %v", got, want)
	}

	requests := server.Requests()
	if got, want := len(requests), 2; got != want {
		t.Fatalf("len(requests) = %d, want %d", got, want)
	}
	if got, want := parseMasterRequest(t, requests[0]).Cursor, "0.0.0.0:0"; got != want {
		t.Fatalf("first cursor = %q, want %q", got, want)
	}
	if got, want := parseMasterRequest(t, requests[1]).Cursor, "2.2.2.2:27016"; got != want {
		t.Fatalf("second cursor = %q, want %q", got, want)
	}
}

func TestStreamEmitsErrorOnceOnPagingFailure(t *testing.T) {
	t.Parallel()

	server := newMasterTestServer(t, func(state *masterTestState, req []byte) [][]byte {
		switch parseMasterRequest(t, req).Cursor {
		case "0.0.0.0:0":
			return [][]byte{buildMasterResponse(masterAddr("4.4.4.4", 27015))}
		case "4.4.4.4:27015":
			return [][]byte{{0x01, 0x02, 0x03}}
		default:
			t.Fatalf("unexpected cursor: %q", parseMasterRequest(t, req).Cursor)
			return nil
		}
	})
	defer server.Close()

	client, err := NewClient(WithBaseAddress(server.Addr().String()))
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	defer client.Close()

	stream, err := client.Stream(context.Background(), Request{Region: RegionUSWest})
	if err != nil {
		t.Fatalf("Stream returned error: %v", err)
	}

	var servers []string
	var streamErr error
	var errCount int
	for result := range stream {
		if result.Err != nil {
			errCount++
			streamErr = result.Err
			continue
		}
		servers = append(servers, result.Server.String())
	}

	if got, want := servers, []string{"4.4.4.4:27015"}; !slices.Equal(got, want) {
		t.Fatalf("servers = %v, want %v", got, want)
	}
	if got, want := errCount, 1; got != want {
		t.Fatalf("errCount = %d, want %d", got, want)
	}
	var masterErr *Error
	if !errors.As(streamErr, &masterErr) {
		t.Fatalf("streamErr type = %T, want *Error", streamErr)
	}
	if got, want := masterErr.Code, ErrorCodePacketHeader; got != want {
		t.Fatalf("streamErr.Code = %q, want %q", got, want)
	}
}

func TestQueryValidationAndTimeoutErrors(t *testing.T) {
	t.Parallel()

	if _, err := NewClient(WithBaseAddress("127.0.0.1:27011")); err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	server := newMasterTestServer(t, func(state *masterTestState, req []byte) [][]byte {
		time.Sleep(120 * time.Millisecond)
		return [][]byte{buildMasterResponse(masterAddr("1.2.3.4", 27015))}
	})
	defer server.Close()

	client, err := NewClient(
		WithBaseAddress(server.Addr().String()),
		WithTimeout(50*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	defer client.Close()

	if _, err := client.Query(context.Background(), Request{
		Region: RegionCustom(0x09),
		Filter: "bad\x00filter",
	}); err == nil {
		t.Fatal("expected Query to reject NUL filter")
	} else {
		var masterErr *Error
		if !errors.As(err, &masterErr) {
			t.Fatalf("expected *Error, got %T", err)
		}
		if got, want := masterErr.Code, ErrorCodeFilter; got != want {
			t.Fatalf("filter error code = %q, want %q", got, want)
		}
	}

	_, err = client.Query(context.Background(), Request{Region: RegionCustom(0x09)})
	if err == nil {
		t.Fatal("expected Query to time out")
	}
	var masterErr *Error
	if !errors.As(err, &masterErr) {
		t.Fatalf("expected *Error, got %T", err)
	}
	if got, want := masterErr.Code, ErrorCodeTimeout; got != want {
		t.Fatalf("timeout error code = %q, want %q", got, want)
	}
}

func TestCloseIdempotent(t *testing.T) {
	t.Parallel()

	server := newMasterTestServer(t, func(state *masterTestState, req []byte) [][]byte {
		return nil
	})
	defer server.Close()

	client, err := NewClient(WithBaseAddress(server.Addr().String()))
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	if err := client.Close(); err != nil {
		t.Fatalf("first Close returned error: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("second Close returned error: %v", err)
	}
}

type parsedMasterRequest struct {
	Region byte
	Cursor string
	Filter string
}

func parseMasterRequest(t *testing.T, req []byte) parsedMasterRequest {
	t.Helper()

	if len(req) < 4 {
		t.Fatalf("request too short: %v", req)
	}
	if req[0] != 0x31 {
		t.Fatalf("request[0] = 0x%X, want 0x31", req[0])
	}

	cursorEnd := bytes.IndexByte(req[2:], 0)
	if cursorEnd < 0 {
		t.Fatalf("request missing cursor terminator: %v", req)
	}
	cursorEnd += 2

	filterStart := cursorEnd + 1
	filterEnd := bytes.IndexByte(req[filterStart:], 0)
	if filterEnd < 0 {
		t.Fatalf("request missing filter terminator: %v", req)
	}
	filterEnd += filterStart

	return parsedMasterRequest{
		Region: req[1],
		Cursor: string(req[2:cursorEnd]),
		Filter: string(req[filterStart:filterEnd]),
	}
}

func buildMasterResponse(addrs ...ServerAddr) []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x66, 0x0A})
	for _, addr := range addrs {
		ip := addr.IP.To4()
		if ip == nil {
			ip = net.IPv4zero.To4()
		}
		buf.Write(ip)
		_ = binary.Write(&buf, binary.BigEndian, addr.Port)
	}
	return buf.Bytes()
}

func masterAddr(ip string, port uint16) ServerAddr {
	return ServerAddr{
		IP:   net.ParseIP(ip).To4(),
		Port: port,
	}
}

type masterTestState struct {
	mu       sync.Mutex
	requests [][]byte
}

func (s *masterTestState) recordRequest(req []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, slices.Clone(req))
}

type masterTestServer struct {
	t       *testing.T
	conn    *net.UDPConn
	handler func(*masterTestState, []byte) [][]byte
	done    chan struct{}
	state   masterTestState
}

func newMasterTestServer(t *testing.T, handler func(*masterTestState, []byte) [][]byte) *masterTestServer {
	t.Helper()

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("ListenUDP returned error: %v", err)
	}
	server := &masterTestServer{
		t:       t,
		conn:    conn,
		handler: handler,
		done:    make(chan struct{}),
	}
	go server.serve()
	return server
}

func (s *masterTestServer) Addr() *net.UDPAddr {
	return s.conn.LocalAddr().(*net.UDPAddr)
}

func (s *masterTestServer) Requests() [][]byte {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	return slices.Clone(s.state.requests)
}

func (s *masterTestServer) Close() {
	close(s.done)
	_ = s.conn.Close()
}

func (s *masterTestServer) serve() {
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
