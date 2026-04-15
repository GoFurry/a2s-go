package errors

import stderrors "errors"

var (
	ErrPacketHeader = stderrors.New("invalid packet header")
	ErrUnsupported  = stderrors.New("unsupported response")
	ErrChallenge    = stderrors.New("challenge handling failed")
	ErrMultiPacket  = stderrors.New("multi packet handling failed")
	ErrDecode       = stderrors.New("decode failed")
)
