import * as dgram from 'dgram'
import { buildPacket, parseHeader, PacketFlag, MAX_PACKET_ID, isPacketCoveredByAck } from './protocol'

const RETRANSMIT_INTERVAL_MS = 150
const MAX_RETRIES = 8
const SESSION_TIMEOUT_MS = 10_000
// Real ATEM hardware streams continuous status/timing traffic, so clients treat silence as a
// dropped connection. We have nothing to stream, so send an empty keepalive packet periodically.
const KEEPALIVE_INTERVAL_MS = 1_000

interface InFlightPacket {
	packetId: number
	payload: Buffer
	sentAt: number
	retries: number
}

interface ClientSession {
	address: string
	port: number
	sessionId: number
	nextPacketId: number
	inFlight: Map<number, InFlightPacket>
	lastActivity: number
}

export class UdpServer {
	private readonly socket = dgram.createSocket('udp4')
	private readonly sessions = new Map<string, ClientSession>()
	private nextSessionId = 1
	private sweepTimer: NodeJS.Timeout | null = null
	private keepaliveTimer: NodeJS.Timeout | null = null

	constructor(
		private readonly port: number,
		private readonly getInitCommands: () => Buffer[],
		private readonly onLog: (msg: string) => void
	) {}

	start(): void {
		this.socket.on('message', (msg, rinfo) => this.onMessage(msg, rinfo))
		this.socket.bind(this.port)
		this.sweepTimer = setInterval(() => this.sweep(), RETRANSMIT_INTERVAL_MS)
		this.keepaliveTimer = setInterval(() => {
			for (const session of this.sessions.values()) {
				this.sendCommands(session, [])
			}
		}, KEEPALIVE_INTERVAL_MS)
	}

	broadcast(commands: Buffer[]): void {
		for (const session of this.sessions.values()) {
			this.sendCommands(session, commands)
		}
	}

	private onMessage(msg: Buffer, rinfo: dgram.RemoteInfo): void {
		const key = `${rinfo.address}:${rinfo.port}`
		const header = parseHeader(msg)

		if (header.flags & PacketFlag.NewSessionId) {
			const session: ClientSession = {
				address: rinfo.address,
				port: rinfo.port,
				sessionId: this.nextSessionId++,
				nextPacketId: 1,
				inFlight: new Map(),
				lastActivity: Date.now(),
			}
			this.sessions.set(key, session)
			this.onLog(`client connected ${key} (session ${session.sessionId})`)

			const response = buildPacket({ flags: PacketFlag.NewSessionId, sessionId: session.sessionId, packetId: 0 })
			this.socket.send(response, session.port, session.address)

			this.sendCommands(session, this.getInitCommands())
			return
		}

		const session = this.sessions.get(key)
		if (!session) return
		session.lastActivity = Date.now()

		if (header.flags & PacketFlag.RetransmitRequest) {
			for (const pkt of session.inFlight.values()) {
				this.socket.send(pkt.payload, session.port, session.address)
			}
		}

		if (header.flags & PacketFlag.AckReply) {
			for (const packetId of session.inFlight.keys()) {
				if (isPacketCoveredByAck(header.ackId, packetId)) {
					session.inFlight.delete(packetId)
				}
			}
		}

		if (header.flags & PacketFlag.AckRequest) {
			const ack = buildPacket({ flags: PacketFlag.AckReply, sessionId: session.sessionId, ackId: header.packetId })
			this.socket.send(ack, session.port, session.address)
		}
	}

	private sendCommands(session: ClientSession, commands: Buffer[]): void {
		const payload = Buffer.concat(commands)
		const packetId = session.nextPacketId
		session.nextPacketId = (session.nextPacketId + 1) % MAX_PACKET_ID

		const packet = buildPacket({ flags: PacketFlag.AckRequest, sessionId: session.sessionId, packetId, payload })
		session.inFlight.set(packetId, { packetId, payload: packet, sentAt: Date.now(), retries: 0 })
		this.socket.send(packet, session.port, session.address)
	}

	private sweep(): void {
		const now = Date.now()
		for (const [key, session] of this.sessions) {
			if (now - session.lastActivity > SESSION_TIMEOUT_MS) {
				this.onLog(`client timed out ${key}`)
				this.sessions.delete(key)
				continue
			}

			for (const pkt of session.inFlight.values()) {
				if (now - pkt.sentAt < RETRANSMIT_INTERVAL_MS) continue
				if (pkt.retries >= MAX_RETRIES) {
					session.inFlight.delete(pkt.packetId)
					continue
				}
				pkt.retries++
				pkt.sentAt = now
				this.socket.send(pkt.payload, session.port, session.address)
			}
		}
	}
}
