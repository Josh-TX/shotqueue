import * as path from 'path'
import express from 'express'
import { UdpServer } from './udpServer'
import { AtemState, NUM_CAMERAS, Snapshot } from './state'
import {
	versionCommand,
	programInputCommand,
	previewInputCommand,
	tallyBySourceCommand,
	initCompleteCommand,
} from './commands'

const ATEM_UDP_PORT = 9910
const WEB_PORT = Number(process.argv[2] ?? process.env.PORT ?? 8080)

function snapshotToCommands(snapshot: Snapshot): Buffer[] {
	return [
		programInputCommand(snapshot.program),
		previewInputCommand(snapshot.preview),
		tallyBySourceCommand(snapshot.tally),
	]
}

let udp: UdpServer

const state = new AtemState((snapshot) => {
	udp.broadcast(snapshotToCommands(snapshot))
})

udp = new UdpServer(
	ATEM_UDP_PORT,
	() => [versionCommand(), ...snapshotToCommands(state.getSnapshot()), initCompleteCommand()],
	(msg) => console.log(`[udp] ${msg}`)
)
udp.start()
console.log(`mock-atem: UDP protocol listening on 0.0.0.0:${ATEM_UDP_PORT}`)

const app = express()
app.use(express.static(path.join(__dirname, '../public')))
app.use(express.json())

app.get('/api/state', (_req, res) => {
	res.json({ ...state.getSnapshot(), numCameras: NUM_CAMERAS })
})

app.post('/api/live/:cam', (req, res) => {
	state.setLive(Number(req.params.cam))
	res.sendStatus(204)
})

app.post('/api/preview/:cam', (req, res) => {
	state.setPreview(Number(req.params.cam))
	res.sendStatus(204)
})

app.post('/api/cut', (_req, res) => {
	state.cut()
	res.sendStatus(204)
})

app.post('/api/fade', (_req, res) => {
	state.fade()
	res.sendStatus(204)
})

app.listen(WEB_PORT, () => {
	console.log(`mock-atem: control UI at http://localhost:${WEB_PORT}`)
})
