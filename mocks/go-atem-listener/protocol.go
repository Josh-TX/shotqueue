package main

import "encoding/binary"

// Wire format ported from sofie-atem-connection's client (src/lib/atemSocketChild.ts).

const (
	flagAckRequest    uint16 = 0x01
	flagNewSessionID  uint16 = 0x02
	flagIsRetransmit  uint16 = 0x04
	flagRetransmitReq uint16 = 0x08
	flagAckReply      uint16 = 0x10
	helloSessionID    uint16 = 0x53ab
	maxPacketID       uint16 = 1 << 15
)

type header struct {
	flags            uint16
	length           uint16
	sessionID        uint16
	ackID            uint16
	retransmitFromID uint16
	packetID         uint16
}

func parseHeader(pkt []byte) header {
	first16 := binary.BigEndian.Uint16(pkt[0:2])
	return header{
		flags:            first16 >> 11,
		length:           first16 & 0x07ff,
		sessionID:        binary.BigEndian.Uint16(pkt[2:4]),
		ackID:            binary.BigEndian.Uint16(pkt[4:6]),
		retransmitFromID: binary.BigEndian.Uint16(pkt[6:8]),
		packetID:         binary.BigEndian.Uint16(pkt[10:12]),
	}
}

func buildPacket(flags, sessionID, ackID, retransmitFromID, packetID uint16, payload []byte) []byte {
	buf := make([]byte, 12+len(payload))
	binary.BigEndian.PutUint16(buf[0:2], (flags<<11)|uint16(12+len(payload)))
	binary.BigEndian.PutUint16(buf[2:4], sessionID)
	binary.BigEndian.PutUint16(buf[4:6], ackID)
	binary.BigEndian.PutUint16(buf[6:8], retransmitFromID)
	binary.BigEndian.PutUint16(buf[10:12], packetID)
	copy(buf[12:], payload)
	return buf
}

// isPacketCoveredByAck mirrors the wraparound-tolerant comparison the real ATEM protocol uses.
func isPacketCoveredByAck(ackID, packetID uint16) bool {
	tolerance := maxPacketID / 2
	shortlyBefore := packetID < ackID && packetID+tolerance > ackID
	shortlyAfter := packetID > ackID && packetID < ackID+tolerance
	beforeWrap := packetID > ackID+tolerance
	return packetID == ackID || ((shortlyBefore || beforeWrap) && !shortlyAfter)
}

type command struct {
	name    string
	payload []byte
}

// parseCommands walks the [len(2)][reserved(2)][name(4)][payload] framing used inside a packet's payload.
func parseCommands(payload []byte) []command {
	var commands []command
	offset := 0
	for offset+8 <= len(payload) {
		length := int(binary.BigEndian.Uint16(payload[offset : offset+2]))
		if length < 8 || offset+length > len(payload) {
			break
		}
		name := string(payload[offset+4 : offset+8])
		commands = append(commands, command{name: name, payload: payload[offset+8 : offset+length]})
		offset += length
	}
	return commands
}
