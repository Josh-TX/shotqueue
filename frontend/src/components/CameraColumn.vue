<template>
  <div
    class="camera-column"
    :class="{ live: camera.status === 'live' }"
    :style="{ width: columnWidth + 'px' }"
    @contextmenu.prevent="openMenu"
  >
    <div class="camera-column-header">
      <div class="header-row name-row">
        <span class="camera-name" :title="name">{{ name }}</span>
        <span class="name-badges">
          <span v-if="camera.generating" class="gen-badge">GENERATING</span>
          <span v-if="camera.status !== 'none'" class="tally-badge" :class="camera.status">{{ tallyLabel }}</span>
          <span v-if="camera.error" class="error-badge">{{ camera.error }}</span>
        </span>
      </div>
      <div class="header-row group-row">
        <GroupSelect :model-value="camera.selectedGroupId ?? null" :groups="camera.groups" @update:model-value="onGroupChange" />
        <button class="icon-btn gear-btn" title="Manage presets" @click="openManage('general')">
          <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor"><path d="M19.14 12.94a7.07 7.07 0 0 0 .06-.94 7.07 7.07 0 0 0-.06-.94l2.03-1.58a.5.5 0 0 0 .12-.64l-1.92-3.32a.5.5 0 0 0-.6-.22l-2.39.96a7.3 7.3 0 0 0-1.62-.94l-.36-2.54a.5.5 0 0 0-.5-.42h-3.84a.5.5 0 0 0-.5.42l-.36 2.54a7.3 7.3 0 0 0-1.62.94l-2.39-.96a.5.5 0 0 0-.6.22L2.7 8.84a.5.5 0 0 0 .12.64l2.03 1.58a7.07 7.07 0 0 0-.06.94 7.07 7.07 0 0 0 .06.94l-2.03 1.58a.5.5 0 0 0-.12.64l1.92 3.32a.5.5 0 0 0 .6.22l2.39-.96c.5.39 1.04.71 1.62.94l.36 2.54a.5.5 0 0 0 .5.42h3.84a.5.5 0 0 0 .5-.42l.36-2.54c.58-.23 1.12-.55 1.62-.94l2.39.96a.5.5 0 0 0 .6-.22l1.92-3.32a.5.5 0 0 0-.12-.64l-2.03-1.58ZM12 15.5a3.5 3.5 0 1 1 0-7 3.5 3.5 0 0 1 0 7Z"/></svg>
        </button>
      </div>
    </div>

    <div class="preset-scroll" :class="{ 'no-overflow': !hasOverflow }" ref="scrollEl">
      <div class="preset-grid" :style="{ gridTemplateColumns: `repeat(${camera.columnCount}, 1fr)` }">
        <PresetThumbnail v-for="p in camera.presets" :key="p.id" :camera="camera" :preset="p" />
        <button class="add-preset-tile" @click="showAddPreset = true">+</button>
      </div>
    </div>

    <AddPresetModal v-if="showAddPreset" :camera="camera" @close="showAddPreset = false" />
    <ManagePresetsModal
      v-if="showManage"
      :camera="camera"
      :unit-width="unitWidth"
      :initial-tab="manageTab"
      @close="showManage = false"
      @add-preset="onAddPresetFromManage"
      @reposition="onRepositionFromManage"
      @open-settings="$emit('open-settings')"
    />
    <ContextMenu v-if="menu" :x="menu.x" :y="menu.y" :items="menuItems" @close="menu = null" />
    <UpdatePresetModal
      v-if="repositioningPreset"
      :camera="camera"
      :preset="repositioningPreset"
      @close="repositioningPreset = null"
    />
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { api } from '../api.js';
import { cameraName } from '../store.js';
import PresetThumbnail from './PresetThumbnail.vue';
import AddPresetModal from './AddPresetModal.vue';
import ManagePresetsModal from './ManagePresetsModal.vue';
import UpdatePresetModal from './UpdatePresetModal.vue';
import GroupSelect from './GroupSelect.vue';
import ContextMenu from './ContextMenu.vue';

const props = defineProps({ camera: Object, unitWidth: Number });
defineEmits(['open-settings']);

// must match .preset-grid gap and .preset-scroll padding (incl. reserved scrollbar gutter) + .camera-column border in style.css
const GRID_GAP = 10;
const COLUMN_OVERHEAD = 12 * 2 + 1;

const name = computed(() => cameraName(props.camera));
const tallyLabel = computed(() => props.camera.status.toUpperCase());
const columnWidth = computed(
  () => props.unitWidth * props.camera.columnCount + GRID_GAP * (props.camera.columnCount - 1) + COLUMN_OVERHEAD,
);

function onGroupChange(id) {
  api.setSelectedGroup(props.camera.cameraNum, id).catch((err) => alert(err.message));
}

const showAddPreset = ref(false);
const showManage = ref(false);
const manageTab = ref('general');

function openManage(tab) {
  manageTab.value = tab;
  showManage.value = true;
}

const menu = ref(null);
function openMenu(e) {
  menu.value = { x: e.clientX, y: e.clientY };
}
const menuItems = computed(() => [
  { label: 'New Preset', action: () => (showAddPreset.value = true) },
  { label: 'General Settings', action: () => openManage('general') },
  { label: 'Manage Groups', action: () => openManage('groups') },
  { label: 'Reorder Presets', action: () => openManage('layout') },
  { divider: true },
  {
    columns: true,
    label: 'Thumbnail Columns',
    count: 6,
    value: props.camera.columnCount,
    onSelect: (n) => api.setColumnCount(props.camera.cameraNum, n).catch((e) => alert(e.message)),
  },
]);

function onAddPresetFromManage() {
  showManage.value = false;
  showAddPreset.value = true;
}

const repositioningPreset = ref(null);
function onRepositionFromManage(preset) {
  showManage.value = false;
  repositioningPreset.value = preset;
}

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
