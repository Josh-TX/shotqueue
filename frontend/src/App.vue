<template>
  <div class="app-shell">
    <Navbar @add-camera="showAddCamera = true" @settings="showSettings = true" />

    <div class="camera-row">
      <CameraColumn v-for="camera in store.cameras" :key="camera.id" :camera="camera" />
      <div v-if="store.cameras.length === 0" class="no-presets" style="margin: auto">
        No cameras yet — click "+ Add Camera" to get started.
      </div>
    </div>

    <AddCameraModal v-if="showAddCamera" @close="showAddCamera = false" />
    <SettingsModal v-if="showSettings" @close="showSettings = false" />
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue';
import Navbar from './components/Navbar.vue';
import CameraColumn from './components/CameraColumn.vue';
import AddCameraModal from './components/AddCameraModal.vue';
import SettingsModal from './components/SettingsModal.vue';
import { store, loadInitial, connectWebSocket, startPositionPolling } from './store.js';

const showAddCamera = ref(false);
const showSettings = ref(false);

onMounted(async () => {
  await loadInitial();
  connectWebSocket();
  startPositionPolling();
});
</script>
