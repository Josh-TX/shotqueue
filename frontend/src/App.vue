<template>
  <div class="app-shell">
    <Navbar @add-camera="showAddCamera = true" @settings="showSettings = true" @versions="showVersions = true" />

    <div class="camera-row">
      <CameraColumn v-for="camera in store.cameras" :key="camera.id" :camera="camera" />
      <div v-if="store.cameras.length === 0" class="no-presets" style="margin: auto">
        No cameras yet — click "+ Add Camera" to get started.
      </div>
    </div>

    <AddCameraModal v-if="showAddCamera" @close="showAddCamera = false" />
    <SettingsModal v-if="showSettings" @close="showSettings = false" />
    <VersionsModal v-if="showVersions" @close="showVersions = false" />
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue';
import Navbar from './components/Navbar.vue';
import CameraColumn from './components/CameraColumn.vue';
import AddCameraModal from './components/AddCameraModal.vue';
import SettingsModal from './components/SettingsModal.vue';
import VersionsModal from './components/VersionsModal.vue';
import { store, loadInitial, connectWebSocket } from './store.js';

const showAddCamera = ref(false);
const showSettings = ref(false);
const showVersions = ref(false);

onMounted(async () => {
  await loadInitial();
  connectWebSocket();
});
</script>
