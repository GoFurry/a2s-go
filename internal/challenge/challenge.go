package challenge

import (
	ierrors "github.com/GoFurry/a2s-go/internal/errors"
	"github.com/GoFurry/a2s-go/internal/protocol"
)

// NoChallenge is the protocol sentinel used in request payloads.
const NoChallenge = 0xFFFFFFFF

// Parse extracts a challenge token from a single packet response.
func Parse(packet []byte) (uint32, error) {
	r := protocol.NewReader(packet)
	if header, ok := r.Int32(); !ok || header != -1 {
		return 0, ierrors.ErrChallenge
	}
	if kind, ok := r.Uint8(); !ok || kind != protocol.HeaderChallenge {
		return 0, ierrors.ErrChallenge
	}
	token, ok := r.Uint32()
	if !ok {
		return 0, ierrors.ErrChallenge
	}
	return token, nil
}
