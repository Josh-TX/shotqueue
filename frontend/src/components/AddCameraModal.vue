<template>
  <Modal title="Add Camera" @close="$emit('close')">
    <div class="field">
      <label>Host</label>
      <input type="text" v-model="host" placeholder="192.168.1.1" @input="resetTest" />
    </div>
    <div class="field">
      <label>Port</label>
      <input type="text" v-model="port" placeholder="80" @input="resetTest" />
    </div>
    <div class="field">
      <label>ATEM tally source number</label>
      <input type="number" v-model.number="tallySource" placeholder="1" />
    </div>
    <div class="field">
      <label>Name</label>
      <input type="text" v-model="name" placeholder="Camera name" />
    </div>

    <img v-if="tested" class="large-thumb" :src="testResult.snapshotUrl" alt="preview" />

    <p v-if="error" class="error-text">{{ error }}</p>

    <div class="modal-actions">
      <button @click="$emit('close')">Cancel</button>
      <button v-if="!tested" class="primary" :disabled="!host.trim() || !port.trim() || testing" @click="test">
        {{ testing ? 'Testing…' : 'Test Camera' }}
      </button>
      <button v-else class="primary" :disabled="!name.trim() || !tallySource || adding" @click="add">
        {{ adding ? 'Adding…' : 'Add Camera' }}
      </button>
    </div>
  </Modal>
</template>

<script setup>
import { ref } from 'vue';
import Modal from './Modal.vue';
import { api } from '../api.js';

const emit = defineEmits(['close']);

const host = ref('');
const port = ref('80');
const tallySource = ref(null);
const name = ref('');
const testing = ref(false);
const adding = ref(false);
const tested = ref(false);
const testResult = ref(null);
const error = ref('');

function resetTest() {
  tested.value = false;
  testResult.value = null;
}

async function test() {
  error.value = '';
  testing.value = true;
  try {
    const result = await api.testCamera(host.value.trim(), port.value.trim());
    testResult.value = result;
    tested.value = true;
    if (!name.value.trim()) name.value = result.suggestedName;
  } catch (e) {
    error.value = e.message;
  } finally {
    testing.value = false;
  }
}

async function add() {
  error.value = '';
  adding.value = true;
  try {
    await api.addCamera(name.value.trim(), host.value.trim(), port.value.trim(), tallySource.value);
    emit('close');
  } catch (e) {
    error.value = e.message;
  } finally {
    adding.value = false;
  }
}
</script>
