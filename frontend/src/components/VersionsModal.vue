<template>
  <Modal title="Versions" wide @close="$emit('close')">
    <div class="versions-shell">
      <div class="versions-sidebar">
        <button
          class="versions-tab"
          :class="{ active: tab === 'named' }"
          @click="tab = 'named'; selectedId = null"
        >
          Named Versions
        </button>
        <button
          class="versions-tab"
          :class="{ active: tab === 'autosave' }"
          @click="tab = 'autosave'; selectedId = null"
        >
          Autosaved Versions
        </button>
        <div class="spacer"></div>
        <button class="versions-tab" :class="{ active: tab === 'save' }" @click="openSave">
          Save Version
        </button>
      </div>

      <div class="versions-content">
        <template v-if="tab === 'save'">
          <div class="version-detail">
            <h3 class="versions-save-title">Save Version</h3>
            <div class="field">
              <label>Version Name</label>
              <input
                type="text"
                v-model="saveName"
                placeholder="Version name"
                autofocus
                @keyup.enter="doSave"
              />
            </div>
            <div class="field">
              <label>or overwrite an existing version</label>
              <div v-if="named.length" class="versions-save-suggestions">
                <button
                  v-for="v in named"
                  :key="v.id"
                  :class="{ 'version-match': isMatch(v) }"
                  @click="saveName = v.name"
                >{{ v.name }}</button>
              </div>
              <div v-else class="metric-block">No named versions yet</div>
            </div>
            <div class="modal-actions">
              <button class="primary" :disabled="!saveName.trim() || saving" @click="doSave">
                {{ matchedNamed ? 'Overwrite' : 'Save' }}
              </button>
            </div>
            <p v-if="saveError" class="error-text">{{ saveError }}</p>
          </div>
        </template>

        <template v-else-if="selected">
          <div class="version-detail">
            <div class="versions-detail-header">
              <h3 class="versions-save-title">
                {{ selected.type === 'named' ? selected.name : formatTimestamp(selected.timestamp) }}
              </h3>
              <div class="versions-detail-header-actions">
                <button v-if="selected.type === 'named'" @click="openOverride(selected)">Override</button>
                <button @click="selectedId = null">Back</button>
              </div>
            </div>
            <div v-if="selected.type === 'named'" class="metric-block versions-last-modified">
              Last modified: {{ formatTimestamp(selected.timestamp) }}
            </div>
            <div v-if="selected.cameras.length === 0" class="metric-block">No cameras in this version</div>
            <div v-for="(c, i) in selected.cameras" :key="i" class="metric-block">
              {{ c.name }}: {{ c.presets.length }} preset{{ c.presets.length === 1 ? '' : 's' }}
            </div>
            <div class="modal-actions">
              <button v-if="selected.type === 'named'" class="delete-version-btn" @click="deleting = selected">Delete</button>
              <label class="regen-checkbox">
                <input type="checkbox" v-model="regenerateThumbnails" />
                Regenerate thumbnails
              </label>
              <button class="primary" :disabled="loading" @click="doLoad">
                {{ loading ? 'Loading…' : 'Load Version' }}
              </button>
            </div>
            <p v-if="loadError" class="error-text">{{ loadError }}</p>
          </div>
        </template>

        <template v-else>
          <template v-if="tab === 'named'">
            <div v-if="named.length === 0" class="metric-block">No named versions yet</div>
            <table v-else class="versions-table">
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Timestamp</th>
                  <th>Cameras</th>
                  <th>Presets</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="v in named" :key="v.id" class="version-row" @click="select(v.id)">
                  <td class="name">{{ v.name }}</td>
                  <td class="metric-block">{{ formatTimestamp(v.timestamp) }}</td>
                  <td class="metric-block">{{ v.cameras.length }}</td>
                  <td class="metric-block">{{ presetCount(v) }}</td>
                </tr>
              </tbody>
            </table>
          </template>
          <template v-else>
            <div v-if="autosaved.length === 0" class="metric-block">No autosaves yet</div>
            <table v-else class="versions-table">
              <thead>
                <tr>
                  <th>Timestamp</th>
                  <th>Cameras</th>
                  <th>Presets</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="v in autosaved" :key="v.id" class="version-row" @click="select(v.id)">
                  <td class="name">{{ formatTimestamp(v.timestamp) }}</td>
                  <td class="metric-block">{{ v.cameras.length }}</td>
                  <td class="metric-block">{{ presetCount(v) }}</td>
                </tr>
              </tbody>
            </table>
          </template>
        </template>
      </div>
    </div>

    <p v-if="listError" class="error-text">{{ listError }}</p>

    <ConfirmDialog
      v-if="deleting"
      title="Delete Version"
      :message="`Delete version &quot;${deleting.name}&quot;? This can't be undone.`"
      @confirm="confirmDelete"
      @cancel="deleting = null"
    />
  </Modal>
</template>

<script setup>
import { computed, ref } from 'vue';
import Modal from './Modal.vue';
import ConfirmDialog from './ConfirmDialog.vue';
import { api } from '../api.js';
import { formatTimestamp } from '../utils.js';

const emit = defineEmits(['close']);

const tab = ref('named');

const versions = ref([]);
const listError = ref('');
api
  .listVersions()
  .then((v) => (versions.value = v))
  .catch((e) => (listError.value = e.message));

const named = computed(() =>
  versions.value.filter((v) => v.type === 'named').slice().sort((a, b) => a.name.localeCompare(b.name))
);
const autosaved = computed(() =>
  versions.value.filter((v) => v.type === 'autosave').slice().sort((a, b) => b.timestamp - a.timestamp)
);

const selectedId = ref(null);
const selected = computed(() => versions.value.find((v) => v.id === selectedId.value) ?? null);
const regenerateThumbnails = ref(true);
function select(id) {
  selectedId.value = id;
  regenerateThumbnails.value = true;
}
function presetCount(v) {
  return v.cameras.reduce((sum, c) => sum + c.presets.length, 0);
}

const saveName = ref('');
const saving = ref(false);
const saveError = ref('');
function openSave() {
  tab.value = 'save';
  saveName.value = '';
  saveError.value = '';
}
function openOverride(v) {
  tab.value = 'save';
  saveName.value = v.name;
  saveError.value = '';
}
function isMatch(v) {
  return saveName.value.trim().toLowerCase() === v.name.toLowerCase();
}
const matchedNamed = computed(() => named.value.find(isMatch) ?? null);

async function doSave() {
  const name = saveName.value.trim();
  if (!name) return;
  saving.value = true;
  saveError.value = '';
  try {
    const v = await api.saveVersion(name);
    const idx = versions.value.findIndex((x) => x.id === v.id);
    if (idx >= 0) versions.value[idx] = v;
    else versions.value.push(v);
    tab.value = 'named';
  } catch (e) {
    saveError.value = e.message;
  } finally {
    saving.value = false;
  }
}

const deleting = ref(null);
async function confirmDelete() {
  try {
    await api.deleteVersion(deleting.value.id);
    versions.value = versions.value.filter((v) => v.id !== deleting.value.id);
    if (selectedId.value === deleting.value.id) selectedId.value = null;
  } catch (e) {
    alert(e.message);
  } finally {
    deleting.value = null;
  }
}

const loading = ref(false);
const loadError = ref('');
async function doLoad() {
  if (!selected.value) return;
  loading.value = true;
  loadError.value = '';
  try {
    await api.loadVersion(selected.value.id, regenerateThumbnails.value);
    emit('close');
  } catch (e) {
    loadError.value = e.message;
  } finally {
    loading.value = false;
  }
}
</script>
