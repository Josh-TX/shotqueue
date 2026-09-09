<template>
  <Modal title="Generate Thumbnails" @close="$emit('close')">
    <p class="gen-desc">This will put all cameras into the "Generating" state, where it triggers a preset, waits for the camera to move there, then takes a snapshot to use as a thumbnail. It does this for all presets.</p>
    <label class="gen-checkbox">
      <input type="checkbox" v-model="onlyMissing" />
      Only Generate Missing Thumbnails
    </label>
    <label class="gen-checkbox">
      <input type="checkbox" v-model="allowLiveMove" />
      Allow moving a live camera
    </label>
    <div class="modal-actions">
      <button :class="allowLiveMove ? 'danger' : 'primary'" :disabled="loading" @click="doGenerate">
        {{ loading ? 'Starting…' : 'Generate' }}
      </button>
    </div>
    <p v-if="error" class="error-text">{{ error }}</p>
  </Modal>
</template>

<script setup>
import { ref } from 'vue';
import Modal from './Modal.vue';
import { api } from '../api.js';

const emit = defineEmits(['close']);

const allowLiveMove = ref(false);
const onlyMissing = ref(true);
const loading = ref(false);
const error = ref('');

async function doGenerate() {
  loading.value = true;
  error.value = '';
  try {
    await api.genThumbnails(allowLiveMove.value, !onlyMissing.value);
    emit('close');
  } catch (e) {
    error.value = e.message;
  } finally {
    loading.value = false;
  }
}
</script>
