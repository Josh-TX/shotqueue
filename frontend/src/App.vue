<template>
  <div class="app-shell">
    <Navbar
      @add-camera="showAddCamera = true"
      @settings="showSettings = true"
      @versions="showVersions = true"
      @gen-thumbnails="showGenThumbnails = true"
    />

    <div class="camera-row" ref="cameraRow">
      <CameraColumn v-for="camera in store.cameras" :key="camera.id" :camera="camera" :unit-width="unitWidth" />
      <div v-if="store.cameras.length === 0" class="no-presets" style="margin: auto">
        No cameras yet — click "+ Add Camera" to get started.
      </div>
    </div>

    <AddCameraModal v-if="showAddCamera" @close="showAddCamera = false" />
    <SettingsModal v-if="showSettings" @close="showSettings = false" />
    <VersionsModal v-if="showVersions" @close="showVersions = false" />
    <GenThumbnailsModal v-if="showGenThumbnails" @close="showGenThumbnails = false" />
    <ToastStack />
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue';
import Navbar from './components/Navbar.vue';
import CameraColumn from './components/CameraColumn.vue';
import AddCameraModal from './components/AddCameraModal.vue';
import SettingsModal from './components/SettingsModal.vue';
import VersionsModal from './components/VersionsModal.vue';
import GenThumbnailsModal from './components/GenThumbnailsModal.vue';
import ToastStack from './components/ToastStack.vue';
import { store, loadInitial, connectWebSocket } from './store.js';

const MIN_UNIT_WIDTH = 80;
const MAX_UNIT_WIDTH = 300;
// must match .preset-grid gap and .preset-scroll padding + .camera-column border in style.css
const GRID_GAP = 10;
const COLUMN_OVERHEAD = 12 * 2 + 1;

const showAddCamera = ref(false);
const showSettings = ref(false);
const showVersions = ref(false);
const showGenThumbnails = ref(false);

const cameraRow = ref(null);
const containerWidth = ref(0);
let resizeObserver;

const totalColumns = computed(() =>
  Math.max(1, store.cameras.reduce((sum, c) => sum + c.columnCount, 0)),
);
const cameraCount = computed(() => Math.max(1, store.cameras.length));

// Each camera column pays COLUMN_OVERHEAD once (padding + border) plus a GRID_GAP
// between each of its cells, not per total column, so that has to be backed out
// before dividing by totalColumns to get a cell width that's consistent regardless
// of how columns are distributed across cameras.
const unitWidth = computed(() => {
  const raw =
    (containerWidth.value - GRID_GAP * (totalColumns.value - cameraCount.value) - COLUMN_OVERHEAD * cameraCount.value) /
    totalColumns.value;
  return Math.min(MAX_UNIT_WIDTH, Math.max(MIN_UNIT_WIDTH, raw));
});

onMounted(async () => {
  await loadInitial();
  connectWebSocket();

  resizeObserver = new ResizeObserver((entries) => {
    containerWidth.value = entries[0].contentRect.width;
  });
  resizeObserver.observe(cameraRow.value);
});

onUnmounted(() => {
  resizeObserver?.disconnect();
});
</script>
