import { reactive } from 'vue';
import { api } from './api.js';

export const store = reactive({
  cameras: [],
  wsConnected: false,
  atemConnected: false,
});

function findCamera(cameraNum) {
  return store.cameras.find((c) => c.cameraNum === cameraNum);
}

function mergeStructural(incoming) {
  store.cameras = incoming.map((cam) => {
    const existing = findCamera(cam.cameraNum);
    if (existing) {
      Object.assign(existing, cam);
      return existing;
    }
    return { ...cam };
  });
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
  };
  return ws;
}

export function activePresetOf(camera) {
  return camera.presets.find((p) => p.id === camera.activePresetId) ?? null;
}

export function cameraName(camera) {
  return `Camera ${camera.cameraNum}`;
}
