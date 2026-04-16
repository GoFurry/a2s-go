package scanner

import (
	"errors"
	"net"
	"slices"
	"testing"

	"github.com/GoFurry/a2s-go/master"
)

func TestParseAddressDefaultsPortAndTrimsSpaces(t *testing.T) {
	t.Parallel()

	addr, err := ParseAddress(" 127.0.0.1 ")
	if err != nil {
		t.Fatalf("ParseAddress returned error: %v", err)
	}

	want := master.ServerAddr{
		IP:   net.IPv4(127, 0, 0, 1).To4(),
		Port: 27015,
	}
	if got := addr.String(); got != want.String() {
		t.Fatalf("addr.String() = %q, want %q", got, want.String())
	}
}

func TestParseAddressesPreservesOrder(t *testing.T) {
	t.Parallel()

	addrs, err := ParseAddresses([]string{"127.0.0.1:27015", "127.0.0.2:27016"})
	if err != nil {
		t.Fatalf("ParseAddresses returned error: %v", err)
	}

	got := []string{addrs[0].String(), addrs[1].String()}
	want := []string{"127.0.0.1:27015", "127.0.0.2:27016"}
	if !slices.Equal(got, want) {
		t.Fatalf("addresses = %v, want %v", got, want)
	}
}

func TestParseAddressesAllowsEmptyInput(t *testing.T) {
	t.Parallel()

	addrs, err := ParseAddresses(nil)
	if err != nil {
		t.Fatalf("ParseAddresses(nil) returned error: %v", err)
	}
	if len(addrs) != 0 {
		t.Fatalf("len(addrs) = %d, want 0", len(addrs))
	}
}

func TestParseAddressRejectsIPv6(t *testing.T) {
	t.Parallel()

	_, err := ParseAddress("[::1]:27015")
	if err == nil {
		t.Fatal("expected ParseAddress to reject IPv6")
	}

	var scannerErr *Error
	if !errors.As(err, &scannerErr) {
		t.Fatalf("expected *Error, got %T", err)
	}
	if got, want := scannerErr.Code, ErrorCodeInput; got != want {
		t.Fatalf("scannerErr.Code = %q, want %q", got, want)
	}
}
