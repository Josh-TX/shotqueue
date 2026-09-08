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
  testCamera: (host, port) => request('POST', '/api/cameras/test', { host, port }),
  addCamera: (name, host, port, tallySource) =>
    request('POST', '/api/cameras', { name, host, port, tallySource }),
  updateCamera: (id, patch) => request('PATCH', `/api/cameras/${id}`, patch),
  deleteCamera: (id) => request('DELETE', `/api/cameras/${id}`),

  getSettings: () => request('GET', '/api/settings'),
  updateSettings: (patch) => request('PUT', '/api/settings', patch),

  addPreset: (camId, name, groupIds) => request('POST', `/api/cameras/${camId}/presets`, { name, groupIds }),
  renamePreset: (camId, presetId, name) => request('PATCH', `/api/cameras/${camId}/presets/${presetId}`, { name }),
  deletePreset: (camId, presetId) => request('DELETE', `/api/cameras/${camId}/presets/${presetId}`),
  triggerPreset: (camId, presetId) => request('POST', `/api/cameras/${camId}/presets/${presetId}/trigger`),
  queuePreset: (camId, presetId) => request('POST', `/api/cameras/${camId}/presets/${presetId}/queue`),
  unqueue: (camId) => request('DELETE', `/api/cameras/${camId}/queue`),

  setSelectedGroup: (camId, groupId) => request('POST', `/api/cameras/${camId}/selected-group`, { groupId }),

  setGroupCount: (camId, count) => request('PATCH', `/api/cameras/${camId}/group-count`, { count }),
  updateGroup: (camId, groupId, patch) => request('PATCH', `/api/cameras/${camId}/groups/${groupId}`, patch),
  setMember: (camId, groupId, presetId, patch) =>
    request('PATCH', `/api/cameras/${camId}/groups/${groupId}/members/${presetId}`, patch),

  listVersions: () => request('GET', '/api/versions'),
  saveVersion: (name) => request('POST', '/api/versions', { name }),
  deleteVersion: (id) => request('DELETE', `/api/versions/${id}`),
  loadVersion: (id, generateThumbnails) =>
    request('POST', `/api/versions/${id}/load`, { generateThumbnails }),

  genThumbnails: (allowLiveMove, includeExisting) =>
    request('POST', '/api/thumbnails/generate', { allowLiveMove, includeExisting }),
};
