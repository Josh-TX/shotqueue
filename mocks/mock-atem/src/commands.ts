import { encodeCommand } from './protocol'

const ME_INDEX = 0

export function versionCommand(): Buffer {
	const payload = Buffer.alloc(4)
	payload.writeUInt32BE(0x00080003, 0) // fake protocol version 8.3, matches modern ATEM fw
	return encodeCommand('_ver', payload)
}

export function programInputCommand(source: number): Buffer {
	const payload = Buffer.alloc(4)
	payload.writeUInt8(ME_INDEX, 0)
	payload.writeUInt16BE(source, 2)
	return encodeCommand('PrgI', payload)
}

export function previewInputCommand(source: number): Buffer {
	const payload = Buffer.alloc(4)
	payload.writeUInt8(ME_INDEX, 0)
	payload.writeUInt16BE(source, 2)
	return encodeCommand('PrvI', payload)
}

export interface TallyState {
	[source: number]: { program: boolean; preview: boolean }
}

export function tallyBySourceCommand(tally: TallyState): Buffer {
	const sources = Object.keys(tally)
		.map(Number)
		.sort((a, b) => a - b)
	const payload = Buffer.alloc(2 + sources.length * 3)
	payload.writeUInt16BE(sources.length, 0)
	sources.forEach((source, i) => {
		const flags = (tally[source].program ? 0x01 : 0) | (tally[source].preview ? 0x02 : 0)
		payload.writeUInt16BE(source, 2 + i * 3)
		payload.writeUInt8(flags, 4 + i * 3)
	})
	return encodeCommand('TlSr', payload)
}

export function initCompleteCommand(): Buffer {
	return encodeCommand('InCm', Buffer.alloc(0))
}
