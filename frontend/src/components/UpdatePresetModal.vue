<template>
  <Modal :title="`Update Position of ${preset.name}`" @close="$emit('close')">
    <p class="help-text">Use an external PTZ controller to move {{ cameraLabel }} to the desired position.</p>
    <p class="refresh-thumb-note">thumbnail refreshed every 500ms</p>
    <img class="large-thumb" :src="currentThumbnailUrl" alt="current position" />

    <p v-if="!canCapture" class="error-text">Camera is currently moving. Wait for it to settle before updating.</p>

    <p v-if="error" class="error-text">{{ error }}</p>

    <div class="modal-actions">
      <button @click="$emit('close')">Cancel</button>
      <button class="primary" :disabled="!canCapture" @click="submit">Update Position</button>
    </div>
  </Modal>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue';
import Modal from './Modal.vue';
import { api } from '../api.js';
import { cameraName } from '../store.js';

const props = defineProps({ camera: Object, preset: Object });
const emit = defineEmits(['close']);

const cameraLabel = computed(() => cameraName(props.camera));
const error = ref('');
const refreshTs = ref(Date.now());

const canCapture = computed(() => !props.camera.triggeringPresetId);
const currentThumbnailUrl = computed(() => `/api/cameras/${props.camera.cameraNum}/snapshot?t=${refreshTs.value}`);

let interval;
onMounted(() => {
  interval = setInterval(() => (refreshTs.value = Date.now()), 500);
});
onUnmounted(() => clearInterval(interval));

async function submit() {
  try {
    await api.updatePresetPosition(props.camera.cameraNum, props.preset.id);
    emit('close');
  } catch (e) {
    error.value = e.message;
  }
}
</script>
