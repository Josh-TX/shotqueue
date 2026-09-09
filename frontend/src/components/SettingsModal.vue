<template>
  <Modal title="Settings" wide @close="$emit('close')">
    <div class="tabbed-modal">
      <div class="tabbed-modal-sidebar">
        <button class="tabbed-modal-tab" :class="{ active: tab === 'general' }" @click="tab = 'general'">
          General
        </button>
        <button class="tabbed-modal-tab" :class="{ active: tab === 'cameras' }" @click="tab = 'cameras'">
          Cameras
        </button>
        <div class="spacer"></div>
        <button class="tabbed-modal-tab" :class="{ active: tab === 'add' }" @click="openAdd">
          Add Camera
        </button>
      </div>

      <div class="tabbed-modal-content">
        <template v-if="tab === 'general'">
          <div class="field">
            <label>Server connection</label>
            <div>
              <span class="ws-dot" :class="{ on: wsConnected && atemConnected, warn: wsConnected && !atemConnected }" />
              {{ connectionText }}
            </div>
          </div>

          <div class="field">
            <label>ATEM host</label>
            <div style="display:flex; gap:8px;">
              <input type="text" v-model="atemHost" placeholder="e.g. 192.168.1.100" />
              <button :disabled="savingAtem" @click="saveAtem">{{ savingAtem ? 'Saving…' : 'Save' }}</button>
            </div>
          </div>

          <p v-if="atemError" class="error-text">{{ atemError }}</p>
        </template>

        <template v-else-if="tab === 'cameras'">
          <div class="detail-header">
            <h3 class="detail-title">Cameras</h3>
            <button @click="openAdd">+ Add Camera</button>
          </div>
          <div v-if="store.cameras.length === 0" class="metric-block">No cameras yet</div>
          <table v-else class="cameras-table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Host</th>
                <th>Port</th>
                <th>Username</th>
                <th>Password</th>
                <th>Tally #</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="c in store.cameras" :key="c.id">
                <td><input type="text" v-model="edits[c.id].name" placeholder="Name" @blur="saveCamera(c.id)" /></td>
                <td><input type="text" v-model="edits[c.id].host" placeholder="Host" @blur="saveCamera(c.id)" /></td>
                <td><input type="text" v-model="edits[c.id].port" placeholder="Port" style="width: 60px" @blur="saveCamera(c.id)" /></td>
                <td><input type="text" v-model="edits[c.id].username" placeholder="(none)" @blur="saveCamera(c.id)" /></td>
                <td><input type="password" v-model="edits[c.id].password" placeholder="(unchanged)" autocomplete="new-password" @blur="saveCamera(c.id)" /></td>
                <td><input type="number" v-model.number="edits[c.id].tallySource" placeholder="Tally #" style="width: 60px" @blur="saveCamera(c.id)" /></td>
                <td class="cameras-table-actions">
                  <button @click="removeCamera(c.id)">Delete</button>
                </td>
              </tr>
            </tbody>
          </table>

          <p v-if="camerasError" class="error-text">{{ camerasError }}</p>
        </template>

        <template v-else>
          <div class="detail-panel">
            <div class="detail-scroll">
              <h3 class="detail-title">Add Camera</h3>
              <div class="field">
                <label>Name</label>
                <input type="text" v-model="addName" placeholder="e.g. Camera 1" />
              </div>
              <div class="field">
                <label>Host</label>
                <input type="text" v-model="addHost" placeholder="e.g. 192.168.1.1" @input="resetTest" />
              </div>
              <div class="field">
                <label>Port</label>
                <input type="text" v-model="addPort" placeholder="e.g. 80" @input="resetTest" />
              </div>
              <div class="field">
                <label>Username</label>
                <input type="text" v-model="addUsername" placeholder="(leave blank if no auth)" @input="resetTest" />
              </div>
              <div class="field">
                <label>Password</label>
                <input type="password" v-model="addPassword" placeholder="(leave blank if no auth)" autocomplete="new-password" @input="resetTest" />
              </div>
              <div class="field">
                <label>ATEM tally source number</label>
                <input type="number" v-model.number="addTallySource" placeholder="e.g. 1" />
              </div>

              <img v-if="tested" class="large-thumb" style="max-width: 300px" :src="testResult.snapshotUrl" alt="preview" />

              <p v-if="addError" class="error-text">{{ addError }}</p>
            </div>

            <div class="modal-actions">
              <button v-if="!tested" class="primary" :disabled="!addHost.trim() || !addPort.trim() || testing" @click="test">
                {{ testing ? 'Testing…' : 'Test Camera' }}
              </button>
              <button v-else class="primary" :disabled="!addName.trim() || !addTallySource || adding" @click="add">
                {{ adding ? 'Adding…' : 'Add Camera' }}
              </button>
            </div>
          </div>
        </template>
      </div>
    </div>
  </Modal>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue';
import Modal from './Modal.vue';
import { api } from '../api.js';
import { store } from '../store.js';

defineEmits(['close']);
const tab = ref('general');

const wsConnected = computed(() => store.wsConnected);
const atemConnected = computed(() => store.atemConnected);
const connectionText = computed(() => {
  if (!wsConnected.value) return 'Websocket disconnected';
  return atemConnected.value ? 'Websocket Connected, ATEM connected' : 'Websocket Connected, ATEM disconnected';
});

const atemHost = ref('');
const savingAtem = ref(false);
const atemError = ref('');

onMounted(async () => {
  try {
    const settings = await api.getSettings();
    atemHost.value = settings.atemHost ?? '';
  } catch (e) {
    atemError.value = e.message;
  }
});

async function saveAtem() {
  atemError.value = '';
  savingAtem.value = true;
  try {
    await api.updateSettings({ atemHost: atemHost.value.trim() });
  } catch (e) {
    atemError.value = e.message;
  } finally {
    savingAtem.value = false;
  }
}

const edits = reactive({});
const camerasError = ref('');

function syncEdits() {
  for (const c of store.cameras) {
    if (!edits[c.id]) {
      edits[c.id] = { name: c.name, host: c.ip, port: c.port ?? '', username: c.username ?? '', password: '', tallySource: c.tallySource ?? null };
    }
  }
}

watch(() => store.cameras.length, syncEdits);
syncEdits();

async function saveCamera(id) {
  camerasError.value = '';
  try {
    const e = edits[id];
    const patch = { name: e.name, host: e.host, port: e.port, username: e.username, tallySource: e.tallySource };
    if (e.password) patch.password = e.password;
    await api.updateCamera(id, patch);
    e.password = '';
  } catch (err) {
    camerasError.value = err.message;
  }
}

async function removeCamera(id) {
  if (!confirm('Delete this camera and all its presets/groups?')) return;
  camerasError.value = '';
  try {
    await api.deleteCamera(id);
    delete edits[id];
  } catch (e) {
    camerasError.value = e.message;
  }
}

const addHost = ref('');
const addPort = ref('80');
const addUsername = ref('');
const addPassword = ref('');
const addTallySource = ref(null);
const addName = ref('');
const testing = ref(false);
const adding = ref(false);
const tested = ref(false);
const testResult = ref(null);
const addError = ref('');

function openAdd() {
  tab.value = 'add';
  addHost.value = '';
  addPort.value = '80';
  addUsername.value = '';
  addPassword.value = '';
  addTallySource.value = null;
  addName.value = '';
  addError.value = '';
  resetTest();
}

function resetTest() {
  tested.value = false;
  testResult.value = null;
}

async function test() {
  addError.value = '';
  testing.value = true;
  try {
    const result = await api.testCamera(addHost.value.trim(), addPort.value.trim(), addUsername.value.trim(), addPassword.value);
    testResult.value = result;
    tested.value = true;
    if (!addName.value.trim()) addName.value = result.suggestedName;
  } catch (e) {
    addError.value = e.message;
  } finally {
    testing.value = false;
  }
}

async function add() {
  addError.value = '';
  adding.value = true;
  try {
    await api.addCamera(
      addName.value.trim(),
      addHost.value.trim(),
      addPort.value.trim(),
      addUsername.value.trim(),
      addPassword.value,
      addTallySource.value,
    );
    tab.value = 'cameras';
  } catch (e) {
    addError.value = e.message;
  } finally {
    adding.value = false;
  }
}
</script>
