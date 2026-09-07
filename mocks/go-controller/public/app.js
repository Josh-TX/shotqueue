const hostInput = document.getElementById('host')
const portInput = document.getElementById('port')
const pollRateInput = document.getElementById('pollRate')
const statusEl = document.getElementById('status')
const snapshotEl = document.getElementById('snapshot')
const positionEl = document.getElementById('position')

const sliders = {
	pan: document.getElementById('panSlider'),
	tilt: document.getElementById('tiltSlider'),
	zoom: document.getElementById('zoomSlider'),
}
const outputs = {
	pan: document.getElementById('panOut'),
	tilt: document.getElementById('tiltOut'),
	zoom: document.getElementById('zoomOut'),
}

for (const axis of Object.keys(sliders)) {
	sliders[axis].addEventListener('input', () => {
		outputs[axis].textContent = `${sliders[axis].value}%`
	})
}

let pollTimer = null

function target() {
	return { host: hostInput.value.trim(), port: portInput.value.trim() }
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

		statusEl.textContent = `connected to ${host}:${port}`
	} catch (err) {
		statusEl.textContent = `error: ${err.message}`
	}
}

document.getElementById('connect').addEventListener('click', () => {
	if (pollTimer) clearInterval(pollTimer)
	const pollRate = Math.max(100, Number(pollRateInput.value) || 1000)
	pollOnce()
	pollTimer = setInterval(pollOnce, pollRate)
})

document.getElementById('setPosition').addEventListener('click', async () => {
	const { host, port } = target()
	const panPct = Number(sliders.pan.value)
	const tiltPct = Number(sliders.tilt.value)
	const zoomPct = Number(sliders.zoom.value)
	try {
		const res = await fetch('/api/position', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ host, port, panPct, tiltPct, zoomPct }),
		})
		if (!res.ok) throw new Error((await res.json()).error || res.statusText)
		statusEl.textContent = `set position sent to ${host}:${port}`
	} catch (err) {
		statusEl.textContent = `error: ${err.message}`
	}
})
