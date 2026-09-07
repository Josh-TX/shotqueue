<template>
  <Modal title="Add Preset" @close="$emit('close')">
    <img v-if="camera.currentThumbnailUrl" class="large-thumb" :src="camera.currentThumbnailUrl" alt="current position" />
    <div v-else class="large-thumb" />

    <p v-if="!canCapture" class="error-text">
      Camera has no usable thumbnail at its current position (it may be mid-move or was nudged by an external
      controller). Wait for it to settle on a known position.
    </p>

    <div class="field">
      <label>Preset name</label>
      <input type="text" v-model="name" autofocus />
    </div>

    <div class="field" v-if="camera.groups.length">
      <label>Add to groups</label>
      <label v-for="g in camera.groups" :key="g.id" style="display:flex;align-items:center;gap:8px;color:var(--text);font-size:13px;margin-bottom:4px;">
        <input type="checkbox" :value="g.id" v-model="groupIds" />
        {{ g.name }}
      </label>
    </div>

    <p v-if="error" class="error-text">{{ error }}</p>

    <div class="modal-actions">
      <button @click="$emit('close')">Cancel</button>
      <button class="primary" :disabled="!name.trim() || !canCapture" @click="submit">Add Preset</button>
    </div>
  </Modal>
</template>

<script setup>
import { computed, ref } from 'vue';
import Modal from './Modal.vue';
import { api } from '../api.js';

const props = defineProps({ camera: Object });
const emit = defineEmits(['close']);

const name = ref(`Preset ${props.camera.presets.length + 1}`);
const groupIds = ref([]);
const error = ref('');

const canCapture = computed(() => !props.camera.triggering && !!props.camera.currentThumbnailUrl);

async function submit() {
  try {
    await api.addPreset(props.camera.id, name.value.trim(), groupIds.value);
    emit('close');
  } catch (e) {
    error.value = e.message;
  }
}
</script>
