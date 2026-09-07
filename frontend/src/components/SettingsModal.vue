<template>
  <Modal title="Settings" @close="$emit('close')">
    <div class="field">
      <label>Server connection</label>
      <div>
        <span class="ws-dot" :class="{ on: wsConnected }" />
        {{ wsConnected ? 'Connected (websocket)' : 'Disconnected' }}
      </div>
    </div>

    <div class="field">
      <label>ATEM host</label>
      <div style="display:flex; gap:8px;">
        <input type="text" v-model="atemHost" placeholder="e.g. 192.168.1.100" />
        <button :disabled="savingAtem" @click="saveAtem">{{ savingAtem ? 'Saving…' : 'Save' }}</button>
      </div>
    </div>

    <div class="field">
      <label>Cameras</label>
      <div v-if="store.cameras.length === 0" class="metric-block">No cameras yet</div>
      <div v-for="c in store.cameras" :key="c.id" class="info-preset-row" style="flex-wrap:wrap; gap:6px;">
        <input type="text" v-model="edits[c.id].name" placeholder="Name" style="width:110px" />
        <input type="text" v-model="edits[c.id].host" placeholder="Host" style="width:110px" />
        <input type="text" v-model="edits[c.id].port" placeholder="Port" style="width:60px" />
        <input type="number" v-model.number="edits[c.id].tallySource" placeholder="Tally #" style="width:70px" />
        <button :disabled="saving[c.id]" @click="saveCamera(c.id)">Save</button>
        <button @click="removeCamera(c.id)">Delete</button>
      </div>
    </div>

    <p v-if="error" class="error-text">{{ error }}</p>
  </Modal>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue';
import Modal from './Modal.vue';
import { api } from '../api.js';
import { store } from '../store.js';

defineEmits(['close']);
const wsConnected = computed(() => store.wsConnected);

const atemHost = ref('');
const savingAtem = ref(false);
const error = ref('');

const edits = reactive({});
const saving = reactive({});

function syncEdits() {
  for (const c of store.cameras) {
    if (!edits[c.id]) {
      edits[c.id] = { name: c.name, host: c.ip, port: c.port ?? '', tallySource: c.tallySource ?? null };
    }
  }
}

watch(() => store.cameras.length, syncEdits);
syncEdits();

onMounted(async () => {
  try {
    const settings = await api.getSettings();
    atemHost.value = settings.atemHost ?? '';
  } catch (e) {
    error.value = e.message;
  }
});

async function saveAtem() {
  error.value = '';
  savingAtem.value = true;
  try {
    await api.updateSettings({ atemHost: atemHost.value.trim() });
  } catch (e) {
    error.value = e.message;
  } finally {
    savingAtem.value = false;
  }
}

async function saveCamera(id) {
  error.value = '';
  saving[id] = true;
  try {
    const e = edits[id];
    await api.updateCamera(id, { name: e.name, host: e.host, port: e.port, tallySource: e.tallySource });
  } catch (err) {
    error.value = err.message;
  } finally {
    saving[id] = false;
  }
}

async function removeCamera(id) {
  if (!confirm('Delete this camera and all its presets/groups?')) return;
  error.value = '';
  try {
    await api.deleteCamera(id);
    delete edits[id];
  } catch (e) {
    error.value = e.message;
  }
}
</script>
