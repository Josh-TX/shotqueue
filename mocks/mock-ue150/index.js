import http from 'node:http'
import { createHash, randomBytes } from 'node:crypto'
import { Jimp, JimpMime } from 'jimp'

const USERNAME = 'admin'
const PASSWORD = '12345'
const BASIC_AUTH_EXPECTED = 'Basic ' + Buffer.from(`${USERNAME}:${PASSWORD}`).toString('base64')

const DIGEST_REALM = 'Control'
const DIGEST_NONCE = randomBytes(16).toString('hex')

const md5 = (str) => createHash('md5').update(str, 'utf8').digest('hex')

function parseDigestHeader(header) {
	const params = {}
	const re = /(\w+)=(?:"([^"]*)"|([^,]+))/g
	let match
	while ((match = re.exec(header))) {
		params[match[1]] = match[2] !== undefined ? match[2] : match[3]
	}
	return params
}

function checkDigestAuth(req) {
	const header = req.headers['authorization']
	if (!header?.startsWith('Digest ')) return false
	const p = parseDigestHeader(header.slice('Digest '.length))
	if (p.username !== USERNAME || p.realm !== DIGEST_REALM || p.nonce !== DIGEST_NONCE) return false
	const ha1 = md5(`${USERNAME}:${DIGEST_REALM}:${PASSWORD}`)
	const ha2 = md5(`${req.method}:${p.uri}`)
	const expected = p.qop ? md5(`${ha1}:${p.nonce}:${p.nc}:${p.cnonce}:${p.qop}:${ha2}`) : md5(`${ha1}:${p.nonce}:${ha2}`)
	return p.response === expected
}

const OUTPUT_WIDTH = 640
const OUTPUT_HEIGHT = 360
const MIN_CROP_FRACTION = 0.2 // fraction of the source image's dimensions shown at full tele zoom

// Real camera range; tilt reuses it too since the documented tilt range doesn't bracket 0x8000 as
// its own center, and exact degree fidelity isn't needed here.
const PAN_MIN = 0x2d09
const PAN_MAX = 0xd2f5
const TILT_MIN = PAN_MIN
const TILT_MAX = PAN_MAX
const ZOOM_MIN = 0x555
const ZOOM_MAX = 0xfff

function parseArgs(argv) {
	let imagePath
	let auth = 'off'
	let port = 8080
	for (const arg of argv) {
		if (arg.startsWith('--auth=')) {
			auth = arg.slice('--auth='.length)
		} else if (arg.startsWith('--port=')) {
			port = parseInt(arg.slice('--port='.length), 10)
		} else if (!arg.startsWith('--')) {
			imagePath = arg
		}
	}
	const authMode = auth === 'basic' || auth === 'digest' ? auth : 'off'
	return { imagePath, authMode, port }
}

const { imagePath, authMode, port } = parseArgs(process.argv.slice(2))

if (!imagePath) {
	console.error('Usage: node index.js <imagePath> [--auth=basic|digest] [--port=8080]')
	process.exit(1)
}

const sourceImage = await Jimp.read(imagePath)

const MOVE_DURATION_MS = 2000

// Each move is {from, to, startTime}; live() collapses it to the current interpolated value and
// clears it once the duration has elapsed, so every read/render always asks for the live value
// instead of trusting a stale target.
let panTiltMove = null
let zoomMove = null
let pan = 0x8000
let tilt = 0x8000
let zoom = ZOOM_MIN

function toHex(value, width) {
	return value.toString(16).toUpperCase().padStart(width, '0')
}

function clamp(value, min, max) {
	return Math.min(Math.max(value, min), max)
}

function normalize(value, min, max) {
	return clamp((value - min) / (max - min), 0, 1)
}

function lerp(from, to, frac) {
	return Math.round(from + (to - from) * frac)
}

function livePanTilt() {
	if (!panTiltMove) return { pan, tilt }
	const frac = clamp((Date.now() - panTiltMove.startTime) / MOVE_DURATION_MS, 0, 1)
	if (frac >= 1) {
		pan = panTiltMove.to.pan
		tilt = panTiltMove.to.tilt
		panTiltMove = null
		return { pan, tilt }
	}
	return {
		pan: lerp(panTiltMove.from.pan, panTiltMove.to.pan, frac),
		tilt: lerp(panTiltMove.from.tilt, panTiltMove.to.tilt, frac),
	}
}

function liveZoom() {
	if (!zoomMove) return zoom
	const frac = clamp((Date.now() - zoomMove.startTime) / MOVE_DURATION_MS, 0, 1)
	if (frac >= 1) {
		zoom = zoomMove.to
		zoomMove = null
		return zoom
	}
	return lerp(zoomMove.from, zoomMove.to, frac)
}

function startPanTiltMove(toPan, toTilt) {
	panTiltMove = { from: livePanTilt(), to: { pan: toPan, tilt: toTilt }, startTime: Date.now() }
}

function startZoomMove(toZoom) {
	zoomMove = { from: liveZoom(), to: toZoom, startTime: Date.now() }
}

async function renderSnapshot() {
	const imgW = sourceImage.width
	const imgH = sourceImage.height

	const { pan: livePan, tilt: liveTilt } = livePanTilt()
	const zoomFrac = normalize(liveZoom(), ZOOM_MIN, ZOOM_MAX)
	const cropScale = 1 - zoomFrac * (1 - MIN_CROP_FRACTION)
	const cropW = Math.max(1, Math.round(imgW * cropScale))
	const cropH = Math.max(1, Math.round(imgH * cropScale))

	const panFrac = normalize(livePan, PAN_MIN, PAN_MAX)
	const tiltFrac = normalize(liveTilt, TILT_MIN, TILT_MAX)
	const centerX = panFrac * imgW
	const centerY = tiltFrac * imgH

	const x = clamp(Math.round(centerX - cropW / 2), 0, imgW - cropW)
	const y = clamp(Math.round(centerY - cropH / 2), 0, imgH - cropH)

	const frame = sourceImage.clone().crop({ x, y, w: cropW, h: cropH }).resize({ w: OUTPUT_WIDTH, h: OUTPUT_HEIGHT })
	return frame.getBuffer(JimpMime.jpeg)
}

function checkAuth(req, res) {
	if (authMode === 'off') return true
	if (authMode === 'basic') {
		if (req.headers['authorization'] === BASIC_AUTH_EXPECTED) return true
		res.writeHead(401, { 'WWW-Authenticate': `Basic realm="${DIGEST_REALM}"` })
		res.end()
		return false
	}
	if (checkDigestAuth(req)) return true
	res.writeHead(401, { 'WWW-Authenticate': `Digest realm="${DIGEST_REALM}", nonce="${DIGEST_NONCE}", qop="auth"` })
	res.end()
	return false
}

function handlePtz(url, res) {
	const cmd = url.searchParams.get('cmd') || ''
	const body = cmd.slice(1) // strip leading '#'

	res.writeHead(200, { 'Content-Type': 'text/plain' })

	if (body === 'PTV') {
		const { pan: livePan, tilt: liveTilt } = livePanTilt()
		res.end('pTV' + toHex(livePan, 4) + toHex(liveTilt, 4) + toHex(liveZoom(), 3) + '555555')
		return
	}
	if (body === 'APC') {
		const { pan: livePan, tilt: liveTilt } = livePanTilt()
		res.end('aPC' + toHex(livePan, 4) + toHex(liveTilt, 4))
		return
	}
	const setPanTilt = /^APC([0-9A-Fa-f]{4})([0-9A-Fa-f]{4})$/.exec(body)
	if (setPanTilt) {
		const toPan = parseInt(setPanTilt[1], 16)
		const toTilt = parseInt(setPanTilt[2], 16)
		startPanTiltMove(toPan, toTilt)
		res.end('aPC' + toHex(toPan, 4) + toHex(toTilt, 4))
		return
	}
	if (body === 'GZ') {
		res.end('gz' + toHex(liveZoom(), 3))
		return
	}
	const setZoom = /^AXZ([0-9A-Fa-f]{3})$/.exec(body)
	if (setZoom) {
		const toZoom = parseInt(setZoom[1], 16)
		startZoomMove(toZoom)
		res.end('axz' + toHex(toZoom, 3))
		return
	}
	res.end('eR1:' + body)
}

async function handleView(url, res) {
	const action = url.searchParams.get('action')
	if (action === 'start' || action === 'stop') {
		res.writeHead(200)
		res.end()
		return
	}
	if (action === 'snapshot') {
		const jpeg = await renderSnapshot()
		res.writeHead(200, { 'Content-Type': 'image/jpeg' })
		res.end(jpeg)
		return
	}
	res.writeHead(400)
	res.end()
}

const server = http.createServer(async (req, res) => {
	console.log(`${req.method} ${req.url}`)
	const url = new URL(req.url, `http://${req.headers.host}`)

	if (!checkAuth(req, res)) return

	try {
		if (url.pathname === '/cgi-bin/aw_ptz') {
			handlePtz(url, res)
		} else if (url.pathname === '/cgi-bin/view.cgi') {
			await handleView(url, res)
		} else {
			res.writeHead(404)
			res.end()
		}
	} catch (err) {
		console.error(err)
		res.writeHead(500)
		res.end()
	}
})

server.listen(port, () => {
	console.log(`mock-ue150 listening on http://localhost:${port}`)
	console.log(`Authentication: ${authMode}`)
	console.log(`image: ${imagePath}`)
})
