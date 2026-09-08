<template>
  <div class="camera-column" :style="{ width: columnWidth + 'px' }">
    <div class="camera-column-header">
      <div class="header-row">
        <span class="camera-name" :title="camera.name">{{ camera.name }}</span>
        <span class="tally-badge" :class="[camera.status, { hidden: camera.status === 'none' }]">{{ tallyLabel }}</span>
      </div>
      <div class="header-row">
        <GroupSelect :model-value="camera.selectedGroupId ?? null" :groups="camera.groups" @update:model-value="onGroupChange" />
        <div class="spacer" />
        <button class="icon-btn" @click="showManage = true">⚙</button>
      </div>
    </div>

    <div class="preset-scroll" :class="{ 'no-overflow': !hasOverflow }" ref="scrollEl">
      <div class="preset-grid" :style="{ gridTemplateColumns: `repeat(${camera.columnCount}, 1fr)` }">
        <PresetThumbnail v-for="p in camera.presets" :key="p.id" :camera="camera" :preset="p" />
        <button class="add-preset-tile" @click="showAddPreset = true">+</button>
      </div>
    </div>

    <AddPresetModal v-if="showAddPreset" :camera="camera" @close="showAddPreset = false" />
    <ManagePresetsModal v-if="showManage" :camera="camera" :unit-width="unitWidth" @close="showManage = false" />
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { api } from '../api.js';
import PresetThumbnail from './PresetThumbnail.vue';
import AddPresetModal from './AddPresetModal.vue';
import ManagePresetsModal from './ManagePresetsModal.vue';
import GroupSelect from './GroupSelect.vue';

const props = defineProps({ camera: Object, unitWidth: Number });

// must match .preset-grid gap and .preset-scroll padding (incl. reserved scrollbar gutter) + .camera-column border in style.css
const GRID_GAP = 10;
const COLUMN_OVERHEAD = 12 * 2 + 1;

const tallyLabel = computed(() => props.camera.status.toUpperCase());
const columnWidth = computed(
  () => props.unitWidth * props.camera.columnCount + GRID_GAP * (props.camera.columnCount - 1) + COLUMN_OVERHEAD,
);

function onGroupChange(id) {
  api.setSelectedGroup(props.camera.id, id).catch((err) => alert(err.message));
}

const showAddPreset = ref(false);
const showManage = ref(false);

const scrollEl = ref(null);
const hasOverflow = ref(false);
let resizeObserver = null;

function checkOverflow() {
  if (scrollEl.value) {
    hasOverflow.value = scrollEl.value.scrollHeight > scrollEl.value.clientHeight;
  }
}

onMounted(() => {
  resizeObserver = new ResizeObserver(checkOverflow);
  resizeObserver.observe(scrollEl.value);
  resizeObserver.observe(scrollEl.value.firstElementChild);
});
onBeforeUnmount(() => resizeObserver?.disconnect());
</script>
