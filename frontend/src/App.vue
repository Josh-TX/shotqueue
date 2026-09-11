<template>
  <div class="app-shell">
    <Navbar
      @settings="showSettings = true"
      @configs="showConfigs = true"
      @gen-thumbnails="showGenThumbnails = true"
      @help="showHelp = true"
    />

    <div class="camera-row" ref="cameraRow">
      <CameraColumn v-for="camera in store.cameras" :key="camera.cameraNum" :camera="camera" :unit-width="unitWidth" />
      <div v-if="store.cameras.length === 0" class="no-presets" style="margin: auto">
        No cameras yet — open Settings to add one.
      </div>
    </div>

    <SettingsModal v-if="showSettings" @close="showSettings = false" />
    <ConfigsModal v-if="showConfigs" @close="showConfigs = false" />
    <GenThumbnailsModal v-if="showGenThumbnails" @close="showGenThumbnails = false" />
    <HelpModal v-if="showHelp" @close="showHelp = false" />
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue';
import Navbar from './components/Navbar.vue';
import CameraColumn from './components/CameraColumn.vue';
import SettingsModal from './components/SettingsModal.vue';
import ConfigsModal from './components/ConfigsModal.vue';
import GenThumbnailsModal from './components/GenThumbnailsModal.vue';
import HelpModal from './components/HelpModal.vue';
import { store, loadInitial, connectWebSocket } from './store.js';

const MIN_UNIT_WIDTH = 80;
const MAX_UNIT_WIDTH = 300;
// must match .preset-grid gap and .preset-scroll padding + .camera-column border in style.css
const GRID_GAP = 10;
const COLUMN_OVERHEAD = 12 * 2 + 1;

const showSettings = ref(false);
const showConfigs = ref(false);
const showGenThumbnails = ref(false);
const showHelp = ref(false);

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
