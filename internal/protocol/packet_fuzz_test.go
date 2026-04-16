package protocol

import "testing"

func FuzzClassifyPacket(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x49})
	f.Add([]byte{0xFE, 0xFF, 0xFF, 0xFF, 0x00})

	f.Fuzz(func(t *testing.T, packet []byte) {
		_, _ = ClassifyPacket(packet)
		_, _ = PeekPayloadHeader(packet)
	})
}
