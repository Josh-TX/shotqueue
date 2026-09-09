<template>
  <Modal title="Configs" wide @close="$emit('close')">
    <div class="tabbed-modal">
      <div class="tabbed-modal-sidebar">
        <button
          class="tabbed-modal-tab"
          :class="{ active: tab === 'named' }"
          @click="tab = 'named'; selectedId = null"
        >
          Named Configs
        </button>
        <button
          class="tabbed-modal-tab"
          :class="{ active: tab === 'autosave' }"
          @click="tab = 'autosave'; selectedId = null"
        >
          Autosaved Configs
        </button>
        <div class="spacer"></div>
        <button class="tabbed-modal-tab" :class="{ active: tab === 'save' }" @click="openSave">
          Save Config
        </button>
      </div>

      <div class="tabbed-modal-content">
        <template v-if="tab === 'save'">
          <div class="detail-panel">
            <h3 class="detail-title">Save Config</h3>
            <div class="field">
              <label>Config Name</label>
              <input
                type="text"
                v-model="saveName"
                placeholder="Config name"
                autofocus
                @keyup.enter="doSave"
              />
            </div>
            <div class="field">
              <label>or overwrite an existing config</label>
              <div v-if="named.length" class="config-save-suggestions">
                <button
                  v-for="c in named"
                  :key="c.id"
                  :class="{ 'config-match': isMatch(c) }"
                  @click="saveName = c.name"
                >{{ c.name }}</button>
              </div>
              <div v-else class="metric-block">No named configs yet</div>
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
          <div class="detail-panel">
            <div class="detail-header">
              <h3 class="detail-title">
                {{ selected.type === 'named' ? selected.name : formatTimestamp(selected.timestamp) }}
              </h3>
              <div class="config-detail-header-actions">
                <button v-if="selected.type === 'named'" @click="openOverride(selected)">Override</button>
                <button @click="selectedId = null">Back</button>
              </div>
            </div>
            <div v-if="selected.type === 'named'" class="metric-block config-last-modified">
              Last modified: {{ formatTimestamp(selected.timestamp) }}
            </div>
            <div v-if="selected.cameras.length === 0" class="metric-block">No cameras in this config</div>
            <div v-for="(c, i) in selected.cameras" :key="i" class="metric-block">
              {{ c.name }}: {{ c.presets.length }} preset{{ c.presets.length === 1 ? '' : 's' }}
            </div>
            <div class="modal-actions">
              <button v-if="selected.type === 'named'" class="delete-config-btn" @click="deleting = selected">Delete</button>
              <label class="gen-checkbox">
                <input type="checkbox" v-model="generateThumbnails" />
                Generate thumbnails
              </label>
              <button class="primary" :disabled="loading" @click="doLoad">
                {{ loading ? 'Loading…' : 'Load Config' }}
              </button>
            </div>
            <p v-if="loadError" class="error-text">{{ loadError }}</p>
          </div>
        </template>

        <template v-else>
          <template v-if="tab === 'named'">
            <div v-if="named.length === 0" class="metric-block">No named configs yet</div>
            <table v-else class="configs-table">
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Timestamp</th>
                  <th>Cameras</th>
                  <th>Presets</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="c in named" :key="c.id" class="config-row" @click="select(c.id)">
                  <td class="name">{{ c.name }}</td>
                  <td class="metric-block">{{ formatTimestamp(c.timestamp) }}</td>
                  <td class="metric-block">{{ c.cameras.length }}</td>
                  <td class="metric-block">{{ presetCount(c) }}</td>
                </tr>
              </tbody>
            </table>
          </template>
          <template v-else>
            <div v-if="autosaved.length === 0" class="metric-block">No autosaves yet</div>
            <table v-else class="configs-table">
              <thead>
                <tr>
                  <th>Timestamp</th>
                  <th>Cameras</th>
                  <th>Presets</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="c in autosaved" :key="c.id" class="config-row" @click="select(c.id)">
                  <td class="name">{{ formatTimestamp(c.timestamp) }}</td>
                  <td class="metric-block">{{ c.cameras.length }}</td>
                  <td class="metric-block">{{ presetCount(c) }}</td>
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
      title="Delete Config"
      :message="`Delete config &quot;${deleting.name}&quot;? This can't be undone.`"
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

const configs = ref([]);
const listError = ref('');
api
  .listConfigs()
  .then((c) => (configs.value = c))
  .catch((e) => (listError.value = e.message));

const named = computed(() =>
  configs.value.filter((c) => c.type === 'named').slice().sort((a, b) => a.name.localeCompare(b.name))
);
const autosaved = computed(() =>
  configs.value.filter((c) => c.type === 'autosave').slice().sort((a, b) => b.timestamp - a.timestamp)
);

const selectedId = ref(null);
const selected = computed(() => configs.value.find((c) => c.id === selectedId.value) ?? null);
const generateThumbnails = ref(true);
function select(id) {
  selectedId.value = id;
  generateThumbnails.value = true;
}
function presetCount(c) {
  return c.cameras.reduce((sum, cam) => sum + cam.presets.length, 0);
}

const saveName = ref('');
const saving = ref(false);
const saveError = ref('');
function openSave() {
  tab.value = 'save';
  saveName.value = '';
  saveError.value = '';
}
function openOverride(c) {
  tab.value = 'save';
  saveName.value = c.name;
  saveError.value = '';
}
function isMatch(c) {
  return saveName.value.trim().toLowerCase() === c.name.toLowerCase();
}
const matchedNamed = computed(() => named.value.find(isMatch) ?? null);

async function doSave() {
  const name = saveName.value.trim();
  if (!name) return;
  saving.value = true;
  saveError.value = '';
  try {
    const c = await api.saveConfig(name);
    const idx = configs.value.findIndex((x) => x.id === c.id);
    if (idx >= 0) configs.value[idx] = c;
    else configs.value.push(c);
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
    await api.deleteConfig(deleting.value.id);
    configs.value = configs.value.filter((c) => c.id !== deleting.value.id);
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
    await api.loadConfig(selected.value.id, generateThumbnails.value);
    emit('close');
  } catch (e) {
    loadError.value = e.message;
  } finally {
    loading.value = false;
  }
}
</script>
