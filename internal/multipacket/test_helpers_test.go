package multipacket

import (
	"encoding/binary"
	"hash/crc32"
	"net"
	"testing"
)

var compressedFixturePayload = []byte("hello from multipacket compressed payload")

var compressedFixtureBzip2 = []byte{
	66, 90, 104, 57, 49, 65, 89, 38, 83, 89, 180, 157, 173, 157,
	0, 0, 8, 17, 128, 64, 0, 47, 110, 222, 32, 32, 0, 49,
	77, 50, 49, 49, 49, 8, 131, 77, 50, 100, 102, 164, 196, 39,
	115, 116, 69, 196, 188, 249, 25, 225, 164, 15, 18, 153, 115, 96,
	234, 133, 65, 54, 135, 197, 220, 145, 78, 20, 36, 45, 39, 107,
	103, 64,
}

func buildSplitPacket(id uint32, total, number uint8, splitSize uint16, payload []byte) []byte {
	packet := make([]byte, 12+len(payload))
	binary.LittleEndian.PutUint32(packet[:4], uint32(0xFFFFFFFE))
	binary.LittleEndian.PutUint32(packet[4:8], id)
	packet[8] = total
	packet[9] = number
	binary.LittleEndian.PutUint16(packet[10:12], splitSize)
	copy(packet[12:], payload)
	return packet
}

func buildCompressedEnvelope(payload []byte, compressed []byte) []byte {
	out := make([]byte, 8+len(compressed))
	binary.LittleEndian.PutUint32(out[:4], uint32(len(payload)))
	binary.LittleEndian.PutUint32(out[4:8], crc32.ChecksumIEEE(payload))
	copy(out[8:], compressed)
	return out
}

func mustListenUDP(t testing.TB) *net.UDPConn {
	t.Helper()

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("ListenUDP returned error: %v", err)
	}
	return conn
}

func writePackets(t testing.TB, conn *net.UDPConn, target *net.UDPAddr, packets ...[]byte) {
	t.Helper()

	for _, packet := range packets {
		if _, err := conn.WriteToUDP(packet, target); err != nil {
			t.Fatalf("WriteToUDP returned error: %v", err)
		}
	}
}
