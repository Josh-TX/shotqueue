<template>
  <Modal title="Camera Info" @close="$emit('close')">
    <div class="field">
      <label>Name</label>
      <div>{{ camera.name }}</div>
    </div>
    <div class="field">
      <label>IP Address</label>
      <div>{{ camera.ip }}</div>
    </div>

    <div class="field">
      <label>Presets</label>
      <div v-if="camera.presets.length === 0" class="metric-block">No presets</div>
      <div v-for="p in camera.presets" :key="p.id" class="info-preset-row">
        <div class="name">{{ p.name }}</div>
        <div class="metric-block">
          Scene: <b>{{ p.scene.takenCount }}</b> takes, <b>{{ formatDuration(p.scene.liveTimeMs) }}</b> live
        </div>
        <div class="metric-block">
          Show: <b>{{ p.show.takenCount }}</b> takes, <b>{{ formatDuration(p.show.liveTimeMs) }}</b> live
        </div>
      </div>
    </div>
  </Modal>
</template>

<script setup>
import Modal from './Modal.vue';
import { formatDuration } from '../utils.js';

defineProps({ camera: Object });
defineEmits(['close']);
</script>
