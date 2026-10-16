async function request(method, path, body) {
  const res = await fetch(path, {
    method,
    headers: body !== undefined ? { 'content-type': 'application/json' } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || `request failed (${res.status})`);
  }
  if (res.status === 204) return null;
  return res.json();
}

export const api = {
  listCameras: () => request('GET', '/api/cameras'),
  testCamera: (host, port, username, password) => request('POST', '/api/cameras/test', { host, port, username, password }),
  setColumnCount: (cameraNum, columnCount) => request('PATCH', `/api/cameras/${cameraNum}`, { columnCount }),

  getSettings: () => request('GET', '/api/settings'),
  updateSettings: (patch) => request('PUT', '/api/settings', patch),
  addCameraSettings: (cameraNum, host, port, username, password) =>
    request('POST', '/api/settings/cameras', { cameraNum, host, port, username, password }),
  updateCameraSettings: (currentCameraNum, patch) =>
    request('PATCH', `/api/settings/cameras/${currentCameraNum}`, patch),
  deleteCameraSettings: (cameraNum) => request('DELETE', `/api/settings/cameras/${cameraNum}`),
  setCameraHidden: (cameraNum, isHidden) => request('PATCH', `/api/cameras/${cameraNum}`, { isHidden }),

  addPreset: (camId, name, groupIds) => request('POST', `/api/cameras/${camId}/presets`, { name, groupIds }),
  renamePreset: (camId, presetId, name) => request('PATCH', `/api/cameras/${camId}/presets/${presetId}`, { name }),
  deletePreset: (camId, presetId) => request('DELETE', `/api/cameras/${camId}/presets/${presetId}`),
  reorderPresets: (camId, order) => request('PATCH', `/api/cameras/${camId}/preset-order`, { order }),
  triggerPreset: (camId, presetId) => request('POST', `/api/cameras/${camId}/presets/${presetId}/trigger`),
  updatePresetPosition: (camId, presetId) => request('POST', `/api/cameras/${camId}/presets/${presetId}/position`),
  lockFocus: (camId, presetId) => request('POST', `/api/cameras/${camId}/presets/${presetId}/focus-lock`),
  unlockFocus: (camId, presetId) => request('DELETE', `/api/cameras/${camId}/presets/${presetId}/focus-lock`),
  queuePreset: (camId, presetId) => request('POST', `/api/cameras/${camId}/presets/${presetId}/queue`),
  unqueue: (camId) => request('DELETE', `/api/cameras/${camId}/queue`),

  setAutoQueue: (camId, { groupId, groupIds, mode }) => request('POST', `/api/cameras/${camId}/selected-group`, { groupId, groupIds, mode }),

  setSequenceMode: (camId, isSequence) => request('PATCH', `/api/cameras/${camId}/sequence-mode`, { isSequence }),
  addGroup: (camId) => request('POST', `/api/cameras/${camId}/groups`),
  deleteGroup: (camId, groupId) => request('DELETE', `/api/cameras/${camId}/groups/${groupId}`),
  updateGroup: (camId, groupId, patch) => request('PATCH', `/api/cameras/${camId}/groups/${groupId}`, patch),
  setMember: (camId, groupId, presetId, patch) =>
    request('PATCH', `/api/cameras/${camId}/groups/${groupId}/members/${presetId}`, patch),

  listConfigs: () => request('GET', '/api/configs'),
  saveConfig: (name) => request('POST', '/api/configs', { name }),
  deleteConfig: (id) => request('DELETE', `/api/configs/${id}`),
  loadConfig: (id, generateThumbnails) =>
    request('POST', `/api/configs/${id}/load`, { generateThumbnails }),

  addScene: () => request('POST', '/api/scenes'),
  renameScene: (id, name) => request('PATCH', `/api/scenes/${id}`, { name }),
  deleteScene: (id) => request('DELETE', `/api/scenes/${id}`),
  duplicateScene: (id) => request('POST', `/api/scenes/${id}/duplicate`),
  reorderScenes: (order) => request('PATCH', '/api/scene-order', { order }),
  executeScene: (id) => request('POST', `/api/scenes/${id}/execute`),
  updateSceneCamera: (id, cameraNum, patch) => request('PATCH', `/api/scenes/${id}/cameras/${cameraNum}`, patch),

  genThumbnails: (allowLiveMove, includeExisting) =>
    request('POST', '/api/thumbnails/generate', { allowLiveMove, includeExisting }),
};
