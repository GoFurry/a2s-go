package multipacket

import "testing"

func FuzzParseHeader(f *testing.F) {
	f.Add([]byte{})
	f.Add(buildSplitPacket(0x01020304, 2, 0, 1248, []byte("part-a")))
	f.Add(buildSplitPacket(compressedFlag|0x01020304, 1, 0, 1248, buildCompressedEnvelope(compressedFixturePayload, compressedFixtureBzip2)))

	f.Fuzz(func(t *testing.T, packet []byte) {
		_, _ = parseHeader(packet)
	})
}

func FuzzReadBzip2(f *testing.F) {
	f.Add(compressedFixtureBzip2, len(compressedFixturePayload))
	f.Add([]byte{}, 0)
	f.Add([]byte{0x42, 0x5A, 0x68}, 8)

	f.Fuzz(func(t *testing.T, payload []byte, size int) {
		if size > maxDecompressedPayloadSize+1024 {
			size = maxDecompressedPayloadSize + 1024
		}
		if size < -(maxDecompressedPayloadSize + 1024) {
			size = -(maxDecompressedPayloadSize + 1024)
		}
		_, _ = readBzip2(payload, size)
	})
}
