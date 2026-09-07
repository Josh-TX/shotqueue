// Wire format ported from this repo's own ATEM client (src/lib/atemSocketChild.ts),
// implemented here from the server's side of the handshake.

export enum PacketFlag {
	AckRequest = 0x01,
	NewSessionId = 0x02,
	IsRetransmit = 0x04,
	RetransmitRequest = 0x08,
	AckReply = 0x10,
}

// Fixed session id every real ATEM client sends its first hello packet with.
export const HELLO_SESSION_ID = 0x53ab
export const MAX_PACKET_ID = 1 << 15

export interface ParsedHeader {
	flags: number
	length: number
	sessionId: number
	ackId: number
	retransmitFromId: number
	packetId: number
}

export function parseHeader(packet: Buffer): ParsedHeader {
	const first16 = packet.readUInt16BE(0)
	return {
		flags: first16 >> 11,
		length: first16 & 0x07ff,
		sessionId: packet.readUInt16BE(2),
		ackId: packet.readUInt16BE(4),
		retransmitFromId: packet.readUInt16BE(6),
		packetId: packet.readUInt16BE(10),
	}
}

export function buildPacket(opts: {
	flags: number
	sessionId: number
	ackId?: number
	retransmitFromId?: number
	packetId?: number
	payload?: Buffer
}): Buffer {
	const payload = opts.payload ?? Buffer.alloc(0)
	const buffer = Buffer.alloc(12 + payload.length, 0)
	buffer.writeUInt16BE((opts.flags << 11) | (12 + payload.length), 0)
	buffer.writeUInt16BE(opts.sessionId, 2)
	buffer.writeUInt16BE(opts.ackId ?? 0, 4)
	buffer.writeUInt16BE(opts.retransmitFromId ?? 0, 6)
	buffer.writeUInt16BE(opts.packetId ?? 0, 10)
	payload.copy(buffer, 12)
	return buffer
}

export function encodeCommand(name: string, payload: Buffer): Buffer {
	const buffer = Buffer.alloc(8 + payload.length, 0)
	buffer.writeUInt16BE(8 + payload.length, 0)
	buffer.write(name, 4, 4, 'ascii')
	payload.copy(buffer, 8)
	return buffer
}

// Same wraparound-tolerant comparison the real client uses to decide if a packetId is covered by an ack.
export function isPacketCoveredByAck(ackId: number, packetId: number): boolean {
	const tolerance = MAX_PACKET_ID / 2
	const shortlyBefore = packetId < ackId && packetId + tolerance > ackId
	const shortlyAfter = packetId > ackId && packetId < ackId + tolerance
	const beforeWrap = packetId > ackId + tolerance
	return packetId === ackId || ((shortlyBefore || beforeWrap) && !shortlyAfter)
}
