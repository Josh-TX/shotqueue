import { reactive } from 'vue';
import { api } from './api.js';

export const store = reactive({
  cameras: [],
  wsConnected: false,
  atemConnected: false,
  toasts: [],
});

let nextToastId = 1;
export function pushToast(message) {
  const id = nextToastId++;
  store.toasts.push({ id, message });
  setTimeout(() => {
    store.toasts = store.toasts.filter((t) => t.id !== id);
  }, 5000);
}

function findCamera(id) {
  return store.cameras.find((c) => c.id === id);
}

function mergeStructural(incoming) {
  const seen = new Set();
  for (const cam of incoming) {
    seen.add(cam.id);
    const existing = findCamera(cam.id);
    if (existing) {
      Object.assign(existing, cam);
    } else {
      store.cameras.push({ ...cam });
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
    store.atemConnected = false;
    setTimeout(connectWebSocket, 1000);
  };
  ws.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    if (msg.type === 'state') {
      mergeStructural(msg.cameras);
      store.atemConnected = msg.atemConnected;
    }
    if (msg.type === 'genComplete') {
      pushToast(`Generated ${msg.generated}, skipped ${msg.skipped}, failed ${msg.failed}`);
    }
  };
  return ws;
}

export function activePresetOf(camera) {
  return camera.presets.find((p) => p.id === camera.activePresetId) ?? null;
}
