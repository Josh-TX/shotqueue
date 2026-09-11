<template>
  <Modal title="Settings" wide @close="$emit('close')">
    <div class="tabbed-modal">
      <div class="tabbed-modal-sidebar">
        <button class="tabbed-modal-tab" :class="{ active: tab === 'cameras' }" @click="tab = 'cameras'">
          Cameras
        </button>
        <button class="tabbed-modal-tab" :class="{ active: tab === 'atem' }" @click="tab = 'atem'">
          ATEM
        </button>
        <div class="spacer"></div>
        <button class="tabbed-modal-tab" :class="{ active: tab === 'add' }" @click="openAdd">
          Add Camera
        </button>
      </div>

      <div class="tabbed-modal-content">
        <template v-if="tab === 'atem'">
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
          <div v-if="settingsCameras.length === 0" class="metric-block">No cameras yet</div>
          <template v-else>
          <p style="margin: 0; color: var(--muted)">Camera visibility is part of the configs, but the connection info is part of the global settings</p>
          <table class="cameras-table">
            <thead>
              <tr>
                <th>Visible</th>
                <th>Camera #</th>
                <th>Host</th>
                <th>Port</th>
                <th>Username</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="c in settingsCameras" :key="c.cameraNum">
                <td style="padding-left: 6px">
                  <input
                    type="checkbox"
                    style="width: 24px; height: 24px"
                    :checked="!edits[c.cameraNum].hidden"
                    @change="edits[c.cameraNum].hidden = !$event.target.checked; saveCamera(c.cameraNum)"
                  />
                </td>
                <td><input type="number" v-model.number="edits[c.cameraNum].cameraNum" placeholder="Camera #" style="width: 70px" @blur="saveCamera(c.cameraNum)" /></td>
                <td><input type="text" v-model="edits[c.cameraNum].host" placeholder="Host" @blur="saveCamera(c.cameraNum)" /></td>
                <td><input type="text" v-model="edits[c.cameraNum].port" placeholder="Port" style="width: 60px" @blur="saveCamera(c.cameraNum)" /></td>
                <td><input type="text" v-model="edits[c.cameraNum].username" placeholder="(none)" @blur="saveCamera(c.cameraNum)" /></td>
                <td class="cameras-table-actions">
                  <button @click="removeCamera(c.cameraNum)">Delete</button>
                </td>
              </tr>
            </tbody>
          </table>
          </template>

          <p v-if="camerasError" class="error-text">{{ camerasError }}</p>
        </template>

        <template v-else>
          <div class="detail-panel">
            <div class="detail-scroll">
              <h3 class="detail-title">Add Camera</h3>
              <div class="field">
                <label>Camera # (also its ATEM tally source number)</label>
                <input type="number" v-model.number="addCameraNum" placeholder="e.g. 1" />
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

              <img v-if="tested" class="large-thumb" style="max-width: 300px" :src="testResult.snapshotUrl" alt="preview" />

              <p v-if="addError" class="error-text">{{ addError }}</p>
            </div>

            <div class="modal-actions">
              <button v-if="!tested" class="primary" :disabled="!addHost.trim() || !addPort.trim() || testing" @click="test">
                {{ testing ? 'Testing…' : 'Test Camera' }}
              </button>
              <button v-else class="primary" :disabled="!addCameraNum || adding" @click="add">
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
import { computed, onMounted, reactive, ref } from 'vue';
import Modal from './Modal.vue';
import { api } from '../api.js';
import { store } from '../store.js';

defineEmits(['close']);
const tab = ref('cameras');

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

const settingsCameras = ref([]);
const edits = reactive({});
const camerasError = ref('');

async function loadSettingsCameras() {
  const settings = await api.getSettings();
  settingsCameras.value = settings.cameras ?? [];
  for (const c of settingsCameras.value) {
    edits[c.cameraNum] = { cameraNum: c.cameraNum, host: c.host, port: c.port ?? '', username: c.username ?? '', hidden: c.hidden };
  }
}

onMounted(() => {
  loadSettingsCameras().catch((e) => (camerasError.value = e.message));
});

async function saveCamera(originalNum) {
  camerasError.value = '';
  try {
    const e = edits[originalNum];
    await api.updateCameraSettings(originalNum, {
      cameraNum: e.cameraNum,
      host: e.host,
      port: e.port,
      username: e.username,
      hidden: e.hidden,
    });
    delete edits[originalNum];
    await loadSettingsCameras();
  } catch (err) {
    camerasError.value = err.message;
  }
}

async function removeCamera(cameraNum) {
  if (!confirm('Delete this camera? Any config/state referencing it will be dropped.')) return;
  camerasError.value = '';
  try {
    await api.deleteCameraSettings(cameraNum);
    delete edits[cameraNum];
    await loadSettingsCameras();
  } catch (e) {
    camerasError.value = e.message;
  }
}

const addCameraNum = ref(null);
const addHost = ref('');
const addPort = ref('80');
const addUsername = ref('');
const addPassword = ref('');
const testing = ref(false);
const adding = ref(false);
const tested = ref(false);
const testResult = ref(null);
const addError = ref('');

function openAdd() {
  tab.value = 'add';
  addCameraNum.value = null;
  addHost.value = '';
  addPort.value = '80';
  addUsername.value = '';
  addPassword.value = '';
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
    await api.addCameraSettings(
      addCameraNum.value,
      addHost.value.trim(),
      addPort.value.trim(),
      addUsername.value.trim(),
      addPassword.value,
    );
    await loadSettingsCameras();
    tab.value = 'cameras';
  } catch (e) {
    addError.value = e.message;
  } finally {
    adding.value = false;
  }
}
</script>
