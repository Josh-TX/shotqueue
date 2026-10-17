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
          <span v-if="camera.autoFocus" class="auto-focus-badge" title="Camera is in auto focus mode">A</span>
          <span v-if="camera.error" class="error-badge">{{ camera.error }}</span>
        </span>
      </div>
      <div class="header-row group-row">
        <AutoQueueSelect
          :model-value="camera.selectedGroupId ?? null"
          :selected-group-ids="camera.selectedGroupIds"
          :selected-mode="camera.selectedMode"
          :last-multiple-mode="camera.lastMultipleMode"
          :groups="camera.groups"
          :is-sequence="camera.isSequence"
          @change="onGroupChange"
        />
        <button class="icon-btn gear-btn" title="Manage presets" @click="openManage('general')">
          <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor"><path d="M19.14 12.94a7.07 7.07 0 0 0 .06-.94 7.07 7.07 0 0 0-.06-.94l2.03-1.58a.5.5 0 0 0 .12-.64l-1.92-3.32a.5.5 0 0 0-.6-.22l-2.39.96a7.3 7.3 0 0 0-1.62-.94l-.36-2.54a.5.5 0 0 0-.5-.42h-3.84a.5.5 0 0 0-.5.42l-.36 2.54a7.3 7.3 0 0 0-1.62.94l-2.39-.96a.5.5 0 0 0-.6.22L2.7 8.84a.5.5 0 0 0 .12.64l2.03 1.58a7.07 7.07 0 0 0-.06.94 7.07 7.07 0 0 0 .06.94l-2.03 1.58a.5.5 0 0 0-.12.64l1.92 3.32a.5.5 0 0 0 .6.22l2.39-.96c.5.39 1.04.71 1.62.94l.36 2.54a.5.5 0 0 0 .5.42h3.84a.5.5 0 0 0 .5-.42l.36-2.54c.58-.23 1.12-.55 1.62-.94l2.39.96a.5.5 0 0 0 .6-.22l1.92-3.32a.5.5 0 0 0-.12-.64l-2.03-1.58ZM12 15.5a3.5 3.5 0 1 1 0-7 3.5 3.5 0 0 1 0 7Z"/></svg>
        </button>
      </div>
    </div>

    <div class="preset-scroll" :class="{ 'no-overflow': !hasOverflow }" ref="scrollEl" :style="{ paddingRight: scrollPadRight + 'px' }" @dragover="onScrollDragOver">
      <div class="preset-grid" :style="{ gridTemplateColumns: `repeat(${camera.columnCount}, 1fr)` }">
        <PresetThumbnail
          v-for="p in displayPresets"
          :key="p.id"
          :camera="camera"
          :preset="p"
          :dragging="dragId === p.id"
          @drag-start="onDragStart(p, $event)"
          @drag-enter="onDragEnter(p)"
          @drag-end="onDragEnd"
        />
        <button class="add-preset-tile" @click="showAddPreset = true">+</button>
      </div>
    </div>

    <AddPresetModal v-if="showAddPreset" :camera="camera" @close="showAddPreset = false" />
    <CameraSettingsModal
      v-if="showManage"
      :camera="camera"
      :unit-width="unitWidth"
      :initial-tab="manageTab"
      @close="showManage = false"
      @add-preset="onAddPresetFromManage"
      @reposition="onRepositionFromManage"
      @open-settings="$emit('open-settings')"
    />
    <ContextMenu v-if="menu" :x="menu.x" :y="menu.y" :items="menuItems" :title="name" @close="menu = null" />
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
import CameraSettingsModal from './CameraSettingsModal.vue';
import UpdatePresetModal from './UpdatePresetModal.vue';
import AutoQueueSelect from './AutoQueueSelect.vue';
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

function onGroupChange(sel) {
  api.setAutoQueue(props.camera.cameraNum, sel).catch((err) => alert(err.message));
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

// drag-to-reorder: local order while dragging so server pushes don't clobber it
const dragId = ref(null);
const dragOrder = ref(null);
const displayPresets = computed(() => dragOrder.value ?? props.camera.presets);

const AUTOSCROLL_MARGIN = 50;
const AUTOSCROLL_MAX_SPEED = 14;
let autoScrollSpeed = 0;
let autoScrollRAF = null;

function autoScrollStep() {
  if (scrollEl.value) scrollEl.value.scrollTop += autoScrollSpeed;
  autoScrollRAF = requestAnimationFrame(autoScrollStep);
}
function stopAutoScroll() {
  autoScrollSpeed = 0;
  if (autoScrollRAF !== null) cancelAnimationFrame(autoScrollRAF);
  autoScrollRAF = null;
}
function onScrollDragOver(e) {
  if (dragId.value === null || !scrollEl.value) return;
  const rect = scrollEl.value.getBoundingClientRect();
  let speed = 0;
  if (e.clientY < rect.top + AUTOSCROLL_MARGIN) {
    speed = -Math.ceil(((rect.top + AUTOSCROLL_MARGIN - e.clientY) / AUTOSCROLL_MARGIN) * AUTOSCROLL_MAX_SPEED);
  } else if (e.clientY > rect.bottom - AUTOSCROLL_MARGIN) {
    speed = Math.ceil(((e.clientY - (rect.bottom - AUTOSCROLL_MARGIN)) / AUTOSCROLL_MARGIN) * AUTOSCROLL_MAX_SPEED);
  }
  autoScrollSpeed = speed;
  if (speed !== 0 && autoScrollRAF === null) autoScrollRAF = requestAnimationFrame(autoScrollStep);
  else if (speed === 0) stopAutoScroll();
}

function onDragStart(p, e) {
  dragId.value = p.id;
  dragOrder.value = [...props.camera.presets];
  e.dataTransfer.effectAllowed = 'move';
}
function onDragEnter(p) {
  if (dragId.value === null || dragId.value === p.id) return;
  const order = [...dragOrder.value];
  const from = order.findIndex((x) => x.id === dragId.value);
  const to = order.findIndex((x) => x.id === p.id);
  order.splice(to, 0, order.splice(from, 1)[0]);
  dragOrder.value = order;
}
function onDragEnd() {
  const order = dragOrder.value;
  dragId.value = null;
  stopAutoScroll();
  if (!order) return;
  const ids = order.map((x) => x.id);
  if (ids.every((id, i) => id === props.camera.presets[i]?.id)) {
    dragOrder.value = null;
    return;
  }
  api.reorderPresets(props.camera.cameraNum, ids)
    .catch((e) => alert(e.message))
    .finally(() => (dragOrder.value = null));
}

const scrollEl = ref(null);
const hasOverflow = ref(false);
// right padding = 12px minus the scrollbar gutter the browser reserves (0 for overlay scrollbars, e.g. Safari)
const scrollPadRight = ref(2);
let resizeObserver = null;

function checkOverflow() {
  if (scrollEl.value) {
    const el = scrollEl.value;
    hasOverflow.value = el.scrollHeight > el.clientHeight;
    scrollPadRight.value = Math.max(0, 12 - (el.offsetWidth - el.clientWidth));
  }
}

onMounted(() => {
  resizeObserver = new ResizeObserver(checkOverflow);
  resizeObserver.observe(scrollEl.value);
  resizeObserver.observe(scrollEl.value.firstElementChild);
});
onBeforeUnmount(() => {
  resizeObserver?.disconnect();
  stopAutoScroll();
});
</script>
