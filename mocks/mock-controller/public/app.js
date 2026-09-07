const STORAGE_KEY = 'mock-controller-cameras'
const POLL_MS = 500
// Must match mock-ue150's MIN_CROP_FRACTION: fraction of the full frame visible at max zoom-in.
const MIN_CROP_FRACTION = 0.2
const MIN_DRAG_FRAC = 0.06
const SETTLE_TIMEOUT_MS = 5000

const tabsEl = document.getElementById('tabs')
const statusEl = document.getElementById('status')
const snapshotEl = document.getElementById('snapshot')
const snapshotWrapEl = document.getElementById('snapshotWrap')
const overlayEl = document.getElementById('dragOverlay')
const boxOverlayEl = document.getElementById('dragBoxOverlay')
const positionEl = document.getElementById('position')
const beginBtn = document.getElementById('beginSetPosition')
const cancelBtn = document.getElementById('cancelSetPosition')

let cameras = loadCameras()
let activeIndex = cameras.length ? 0 : -1
let pollTimer = null
let mode = 'idle' // idle | settling | dragging
let settleReadings = []
let settleDeadline = 0
let dragStart = null
let dragging = false

function loadCameras() {
	try {
		const raw = localStorage.getItem(STORAGE_KEY)
		return raw ? JSON.parse(raw) : []
	} catch {
		return []
	}
}

function saveCameras() {
	localStorage.setItem(STORAGE_KEY, JSON.stringify(cameras))
}

function target() {
	return cameras[activeIndex] || { host: '', port: '' }
}

function clamp(v, min, max) {
	return Math.min(max, Math.max(min, v))
}

function renderTabs() {
	tabsEl.innerHTML = ''
	cameras.forEach((cam, i) => {
		const tab = document.createElement('div')
		tab.className = 'tab' + (i === activeIndex ? ' active' : '')

		const label = document.createElement('span')
		label.textContent = `Camera ${i + 1}`
		tab.appendChild(label)

		const close = document.createElement('span')
		close.className = 'close'
		close.textContent = '×'
		close.addEventListener('click', (e) => {
			e.stopPropagation()
			removeCamera(i)
		})
		tab.appendChild(close)

		tab.addEventListener('click', () => switchTo(i))
		tabsEl.appendChild(tab)
	})

	const addTab = document.createElement('div')
	addTab.className = 'tab add'
	addTab.textContent = '+'
	addTab.addEventListener('click', addCamera)
	tabsEl.appendChild(addTab)
}

function addCamera() {
	const host = window.prompt('Camera host', 'localhost')
	if (host === null) return
	const port = window.prompt('Camera port', '8080')
	if (port === null) return
	cameras.push({ host: host.trim(), port: port.trim() })
	saveCameras()
	switchTo(cameras.length - 1)
}

function removeCamera(i) {
	const wasActive = i === activeIndex
	cameras.splice(i, 1)
	saveCameras()

	if (cameras.length === 0) {
		exitSetPositionMode()
		activeIndex = -1
		stopPolling()
		renderTabs()
		beginBtn.disabled = true
		statusEl.textContent = 'no cameras — click + to add one'
		snapshotEl.removeAttribute('src')
		positionEl.textContent = '-'
		return
	}

	if (wasActive) {
		switchTo(Math.min(i, cameras.length - 1))
		return
	}

	if (i < activeIndex) activeIndex -= 1
	renderTabs()
}

function switchTo(i) {
	exitSetPositionMode()
	activeIndex = i
	renderTabs()
	beginBtn.disabled = activeIndex === -1
	startPolling()
}

function stopPolling() {
	if (pollTimer) clearInterval(pollTimer)
	pollTimer = null
}

function startPolling() {
	stopPolling()
	if (activeIndex === -1) return
	pollOnce()
	pollTimer = setInterval(pollOnce, POLL_MS)
}

async function pollOnce() {
	const { host, port } = target()
	try {
		const posRes = await fetch(`/api/position?host=${encodeURIComponent(host)}&port=${encodeURIComponent(port)}`)
		if (!posRes.ok) throw new Error((await posRes.json()).error || posRes.statusText)
		const pos = await posRes.json()
		positionEl.textContent =
			`pan:  0x${pos.pan.toString(16)}\n` + `tilt: 0x${pos.tilt.toString(16)}\n` + `zoom: 0x${pos.zoom.toString(16)}`

		snapshotEl.src = `/api/snapshot?host=${encodeURIComponent(host)}&port=${encodeURIComponent(port)}&t=${Date.now()}`

		if (mode === 'idle') {
			statusEl.textContent = `connected to ${host}:${port}`
		}

		checkSettle(pos)
	} catch (err) {
		statusEl.textContent = `error: ${err.message}`
	}
}

function checkSettle(pos) {
	if (mode !== 'settling') return
	settleReadings.push(pos)
	if (settleReadings.length > 2) settleReadings.shift()
	const stable =
		settleReadings.length === 2 &&
		settleReadings[0].pan === settleReadings[1].pan &&
		settleReadings[0].tilt === settleReadings[1].tilt &&
		settleReadings[0].zoom === settleReadings[1].zoom
	if (stable || Date.now() > settleDeadline) {
		mode = 'dragging'
		statusEl.textContent = 'drag on the snapshot to set position'
	}
}

async function sendPosition(panPct, tiltPct, zoomPct) {
	const { host, port } = target()
	const res = await fetch('/api/position', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ host, port, panPct, tiltPct, zoomPct }),
	})
	if (!res.ok) throw new Error((await res.json()).error || res.statusText)
	return res.json()
}

function exitSetPositionMode() {
	mode = 'idle'
	dragging = false
	dragStart = null
	overlayEl.style.display = 'none'
	overlayEl.classList.remove('too-small')
	boxOverlayEl.style.display = 'none'
	cancelBtn.style.display = 'none'
	if (activeIndex !== -1) beginBtn.disabled = false
}

beginBtn.addEventListener('click', async () => {
	if (mode !== 'idle' || activeIndex === -1) return
	mode = 'settling'
	settleReadings = []
	settleDeadline = Date.now() + SETTLE_TIMEOUT_MS
	beginBtn.disabled = true
	cancelBtn.style.display = 'inline'
	statusEl.textContent = 'zooming out...'
	try {
		// Center pan/tilt, zoom all the way out, so the snapshot shows the full frame to drag on.
		await sendPosition(50, 50, 0)
	} catch (err) {
		statusEl.textContent = `error: ${err.message}`
		exitSetPositionMode()
	}
})

cancelBtn.addEventListener('click', () => {
	exitSetPositionMode()
	statusEl.textContent = 'set position cancelled'
})

function pointFromEvent(e) {
	const rect = snapshotEl.getBoundingClientRect()
	const x = clamp(((e.clientX - rect.left) / rect.width) * 640, 0, 640)
	const y = clamp(((e.clientY - rect.top) / rect.height) * 360, 0, 360)
	return { x, y }
}

// Raw bounding box of the drag, start point to current point.
function bboxFromDrag(start, current) {
	const left = Math.min(start.x, current.x)
	const right = Math.max(start.x, current.x)
	const top = Math.min(start.y, current.y)
	const bottom = Math.max(start.y, current.y)
	return { x: left, y: top, w: right - left, h: bottom - top }
}

// The bbox letterboxed down to 16:9 (the snapshot's aspect ratio) about its center.
function cropToAspect(bbox) {
	const cx = bbox.x + bbox.w / 2
	const cy = bbox.y + bbox.h / 2
	let w = bbox.w
	let h = bbox.h
	if (w <= 0 || h <= 0) return { x: cx, y: cy, w: 0, h: 0 }

	if (w / h > 16 / 9) {
		w = h * (16 / 9)
	} else {
		h = w * (9 / 16)
	}
	return {
		x: clamp(cx - w / 2, 0, 640 - w),
		y: clamp(cy - h / 2, 0, 360 - h),
		w,
		h,
	}
}

function updateOverlay(el, rect) {
	const bounds = snapshotEl.getBoundingClientRect()
	const scaleX = bounds.width / 640
	const scaleY = bounds.height / 360
	el.style.display = 'block'
	el.style.left = `${rect.x * scaleX}px`
	el.style.top = `${rect.y * scaleY}px`
	el.style.width = `${rect.w * scaleX}px`
	el.style.height = `${rect.h * scaleY}px`
}

function updateDragOverlays(bbox) {
	const rect = cropToAspect(bbox)
	updateOverlay(boxOverlayEl, bbox)
	updateOverlay(overlayEl, rect)
	overlayEl.classList.toggle('too-small', rect.w / 640 < MIN_DRAG_FRAC)
	return rect
}

snapshotWrapEl.addEventListener('mousedown', (e) => {
	if (mode !== 'dragging') return
	e.preventDefault()
	dragging = true
	dragStart = pointFromEvent(e)
	updateDragOverlays({ x: dragStart.x, y: dragStart.y, w: 0, h: 0 })
})

window.addEventListener('mousemove', (e) => {
	if (!dragging) return
	updateDragOverlays(bboxFromDrag(dragStart, pointFromEvent(e)))
})

window.addEventListener('mouseup', async (e) => {
	if (!dragging) return
	dragging = false
	const rect = cropToAspect(bboxFromDrag(dragStart, pointFromEvent(e)))
	overlayEl.style.display = 'none'
	overlayEl.classList.remove('too-small')
	boxOverlayEl.style.display = 'none'

	if (rect.w / 640 < MIN_DRAG_FRAC) {
		exitSetPositionMode()
		statusEl.textContent = 'set position cancelled'
		return
	}

	const cropScale = rect.w / 640
	const zoomFrac = clamp((1 - cropScale) / (1 - MIN_CROP_FRACTION), 0, 1)
	const panFrac = (rect.x + rect.w / 2) / 640
	const tiltFrac = (rect.y + rect.h / 2) / 360

	mode = 'idle'
	beginBtn.disabled = false
	cancelBtn.style.display = 'none'
	try {
		await sendPosition(panFrac * 100, tiltFrac * 100, zoomFrac * 100)
		statusEl.textContent = 'position set'
	} catch (err) {
		statusEl.textContent = `error: ${err.message}`
	}
})

renderTabs()
beginBtn.disabled = activeIndex === -1
if (activeIndex !== -1) startPolling()
else statusEl.textContent = 'no cameras — click + to add one'
