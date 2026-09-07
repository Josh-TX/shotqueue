import { reactive } from 'vue';
import { api } from './api.js';

export const store = reactive({
  cameras: [],
  wsConnected: false,
});

const lastPoll = {}; // camera id -> timestamp of last position poll

function findCamera(id) {
  return store.cameras.find((c) => c.id === id);
}

function mergeStructural(incoming) {
  const seen = new Set();
  for (const cam of incoming) {
    seen.add(cam.id);
    const existing = findCamera(cam.id);
    if (existing) {
      const wasTriggering = existing.triggering;
      Object.assign(existing, cam); // activePresetId stays untouched (not in `cam`)
      // The websocket's "triggering just ended" notice can arrive before the
      // next scheduled fast-poll tick. Poll immediately so the position (and
      // therefore the active-preset border) updates without a gray gap.
      if (wasTriggering && !existing.triggering) {
        lastPoll[existing.id] = Date.now();
        pollPosition(existing);
      }
    } else {
      store.cameras.push({ ...cam, activePresetId: null });
    }
  }
  store.cameras = store.cameras.filter((c) => seen.has(c.id));
}

export async function loadInitial() {
  const cameras = await api.listCameras();
  store.cameras = cameras;
}

export function connectWebSocket() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  const ws = new WebSocket(`${proto}://${location.host}/ws`);
  ws.onopen = () => (store.wsConnected = true);
  ws.onclose = () => {
    store.wsConnected = false;
    setTimeout(connectWebSocket, 1000);
  };
  ws.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    if (msg.type === 'state') mergeStructural(msg.cameras);
  };
  return ws;
}

async function pollPosition(camera) {
  try {
    const { activePresetId, triggering, thumbnailUrl } = await api.getPosition(camera.id);
    camera.activePresetId = activePresetId;
    camera.triggering = triggering;
    camera.currentThumbnailUrl = thumbnailUrl;
  } catch {
    // camera might have just been deleted; next structural merge will drop it
  }
}

export function startPositionPolling() {
  setInterval(() => {
    const now = Date.now();
    for (const camera of store.cameras) {
      const interval = camera.triggering ? 100 : 1000;
      if (now - (lastPoll[camera.id] ?? 0) >= interval) {
        lastPoll[camera.id] = now;
        pollPosition(camera);
      }
    }
  }, 100);
}

export function activePresetOf(camera) {
  return camera.presets.find((p) => p.id === camera.activePresetId) ?? null;
}
