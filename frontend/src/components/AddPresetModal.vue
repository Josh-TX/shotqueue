<template>
  <Modal title="Add Preset" @close="$emit('close')">
    <a class="refresh-thumb-link" @click="refreshThumbnail">refresh thumbnail</a>
    <img class="large-thumb" :src="currentThumbnailUrl" alt="current position" />

    <p v-if="!canCapture" class="error-text">Camera is currently moving. Wait for it to settle before adding a preset.</p>

    <div class="field">
      <label>Preset name</label>
      <input type="text" v-model="name" autofocus />
    </div>

    <div class="field" v-if="camera.groups.length">
      <label>Add to groups</label>
      <div class="add-preset-group-checks">
        <input
          v-for="(g, i) in camera.groups"
          :key="g.id"
          type="checkbox"
          class="g-checkbox"
          :style="{ '--group-color': groupColor(i) }"
          :title="g.name"
          :value="g.id"
          v-model="groupIds"
        />
      </div>
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
import { groupColor } from '../colors.js';

const props = defineProps({ camera: Object });
const emit = defineEmits(['close']);

const name = ref(`Preset ${props.camera.presets.length + 1}`);
const groupIds = ref([]);
const error = ref('');
const refreshTs = ref(Date.now());

const canCapture = computed(() => !props.camera.triggering);
const currentThumbnailUrl = computed(() => `/api/cameras/${props.camera.id}/snapshot?t=${refreshTs.value}`);

function refreshThumbnail() {
  refreshTs.value = Date.now();
}

async function submit() {
  try {
    await api.addPreset(props.camera.id, name.value.trim(), groupIds.value);
    emit('close');
  } catch (e) {
    error.value = e.message;
  }
}
</script>
