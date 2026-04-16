package protocol

import (
	"encoding/binary"
	"testing"
)

func TestApplyChallengeOverwritesInfoToken(t *testing.T) {
	t.Parallel()

	req := BuildInfoRequest()
	req = ApplyChallenge(req, 0x01020304)
	req = ApplyChallenge(req, 0x0A0B0C0D)

	if got, want := len(req), infoRequestBaseLen+4; got != want {
		t.Fatalf("len(req) = %d, want %d", got, want)
	}
	if got, want := binary.LittleEndian.Uint32(req[len(req)-4:]), uint32(0x0A0B0C0D); got != want {
		t.Fatalf("challenge = 0x%X, want 0x%X", got, want)
	}
}
