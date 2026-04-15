package main

import (
	"bytes"
	"fmt"
	"log"
	"net"
)

func main() {
	addr, err := net.ResolveUDPAddr("udp4", ":27011")
	if err != nil {
		log.Fatal(err)
	}

	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	fmt.Printf("fake master listening on %s\n", conn.LocalAddr().String())

	buf := make([]byte, 2048)

	for {
		n, client, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Fatal(err)
		}

		packet := append([]byte(nil), buf[:n]...)
		query, err := parseQuery(packet)
		if err != nil {
			fmt.Printf("recv invalid query from %s: err=%v raw=%x\n", client.String(), err, packet)
		} else {
			fmt.Printf(
				"recv query from %s: region=0x%02x cursor=%s filter=%q raw=%x\n",
				client.String(),
				query.Region,
				query.Cursor,
				query.Filter,
				packet,
			)
		}

		// FF FF FF FF 66 0A + [ip(4)+port(2)]... + terminal 0.0.0.0:0
		resp := []byte{
			0xFF, 0xFF, 0xFF, 0xFF, 0x66, 0x0A,

			127, 0, 0, 1, 0x69, 0x87, // 127.0.0.1:27015
			127, 0, 0, 1, 0x69, 0x88, // 127.0.0.1:27016

			0, 0, 0, 0, 0, 0, // terminator
		}

		fmt.Printf(
			"send response to %s: servers=[127.0.0.1:27015 127.0.0.1:27016] done=true raw=%x\n",
			client.String(),
			resp,
		)

		if _, err := conn.WriteToUDP(resp, client); err != nil {
			log.Printf("write response failed: %v", err)
		}
	}
}

type queryPacket struct {
	Region byte
	Cursor string
	Filter string
}

func parseQuery(packet []byte) (queryPacket, error) {
	if len(packet) < 4 {
		return queryPacket{}, fmt.Errorf("packet too short: %d", len(packet))
	}
	if packet[0] != 0x31 {
		return queryPacket{}, fmt.Errorf("unexpected packet type 0x%02x", packet[0])
	}

	cursorEnd := bytes.IndexByte(packet[2:], 0)
	if cursorEnd < 0 {
		return queryPacket{}, fmt.Errorf("missing cursor terminator")
	}
	cursorEnd += 2

	filterStart := cursorEnd + 1
	filterEnd := bytes.IndexByte(packet[filterStart:], 0)
	if filterEnd < 0 {
		return queryPacket{}, fmt.Errorf("missing filter terminator")
	}
	filterEnd += filterStart

	return queryPacket{
		Region: packet[1],
		Cursor: string(packet[2:cursorEnd]),
		Filter: string(packet[filterStart:filterEnd]),
	}, nil
}
