package atem

import "encoding/binary"

// Wire format ported from mocks/go-atem-listener-v2, which itself follows
// sofie-atem-connection's client (src/lib/atemSocketChild.ts).

const (
	atemPort    = 9910
	maxPacketID = 1 << 15
)

const (
	flagAckRequest   uint16 = 0x01
	flagNewSessionID uint16 = 0x02
	flagAckReply     uint16 = 0x10
)

// commandConnectHello is the fixed COMMAND_CONNECT_HELLO packet real ATEM firmware expects to
// start a session.
var commandConnectHello = []byte{
	0x10, 0x14, 0x53, 0xab, 0x00, 0x00, 0x00, 0x00, 0x00, 0x3a, 0x00, 0x00,
	0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
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

// isPacketCoveredByAck mirrors the wraparound-tolerant packet-ID comparison the ATEM protocol
// uses to tell whether a given packet ID was already covered by an earlier ACK.
func isPacketCoveredByAck(ackID, packetID uint16) bool {
	const tolerance = maxPacketID / 2
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
