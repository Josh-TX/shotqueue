<template>
  <Modal title="Generate Thumbnails" @close="$emit('close')">
    <label class="gen-checkbox">
      <input type="checkbox" v-model="allowLiveMove" />
      Allow moving a live camera
    </label>
    <label class="gen-checkbox">
      <input type="checkbox" v-model="onlyMissing" />
      Only Generate Missing Thumbnails
    </label>
    <div class="modal-actions">
      <button class="primary" :disabled="loading" @click="doGenerate">
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
