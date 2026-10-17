<template>
  <Modal title="Manage Scenes" tall @close="$emit('close')">
    <div class="scenes-modal">
      <div class="scenes-sidebar">
        <div class="scenes-sidebar-list">
          <button
            v-for="sc in store.scenes"
            :key="sc.id"
            class="scene-tab"
            :class="{ active: sc.id === selected?.id, dragging: dragId === sc.id }"
            draggable="true"
            @click="select(sc.id)"
            @dragstart="onDragStart(sc, $event)"
            @dragenter.prevent="onDragEnter(sc)"
            @dragover.prevent
            @dragend="onDragEnd"
          >{{ sc.name }}</button>
        </div>
        <button @click="addScene">Add Scene</button>
      </div>

      <div class="scenes-content">
        <div v-if="!selected" class="metric-block">{{ store.scenes.length ? 'Select a scene' : 'No scenes yet' }}</div>
        <template v-else>
          <div class="scene-header">
            <input type="text" :value="selected.name" @blur="rename($event)" @keyup.enter="$event.target.blur()" />
            <button class="primary" @click="execute">Execute</button>
            <button class="icon-btn" title="More" @click="openMenu">
              <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor"><circle cx="12" cy="5" r="2" /><circle cx="12" cy="12" r="2" /><circle cx="12" cy="19" r="2" /></svg>
            </button>
          </div>

          <div v-if="rows.length === 0" class="metric-block">No cameras yet</div>
          <div v-for="row in rows" :key="row.camera.cameraNum" class="scene-camera-row">
            <label class="scene-camera-info">
              <span class="scene-camera-name">{{ cameraName(row.camera) }}</span>
              <span class="scene-visible">
                <input type="checkbox" :checked="!row.entry.isHidden" @change="patch(row, { isHidden: !$event.target.checked })" />
                Visible
              </span>
            </label>
            <AutoQueueSelect
              :model-value="row.entry.groupId ?? null"
              :selected-group-ids="row.entry.groupIds"
              :selected-mode="row.entry.groupMode"
              :last-multiple-mode="row.camera.lastMultipleMode"
              :groups="row.camera.groups"
              :is-sequence="row.camera.isSequence"
              @change="patch(row, { groupId: $event.groupId, groupIds: $event.groupIds, groupMode: $event.mode })"
            />
            <div v-for="kind in kinds" :key="kind.key" class="scene-slot">
              <ManagePresetTile v-if="slotPreset(row, kind)" :preset="slotPreset(row, kind)" :class="kind.border">
                <template #overlay>
                  <button type="button" class="scene-slot-clear" title="Clear" @click="patch(row, { [kind.key]: null })">&times;</button>
                </template>
              </ManagePresetTile>
              <button v-else type="button" class="add-preset-tile scene-slot-add" @click="openPicker(row, kind)">
                <span class="scene-slot-plus">+</span>
                <span class="scene-slot-label">{{ kind.label }}</span>
              </button>
            </div>
          </div>
        </template>
      </div>

      <div v-if="picker && pickerCamera" class="scene-picker">
        <h3 class="scene-picker-title">{{ cameraName(pickerCamera) }} — {{ picker.kind.label }}</h3>
        <div class="manage-preset-grid">
          <div v-for="p in pickerCamera.presets" :key="p.id" class="scene-picker-tile" @click="pick(p)">
            <ManagePresetTile :preset="p" />
          </div>
          <div v-if="pickerCamera.presets.length === 0" class="no-presets">No presets yet</div>
        </div>
      </div>
    </div>

    <ContextMenu v-if="menu" :x="menu.x" :y="menu.y" :items="menuItems" @close="menu = null" />
  </Modal>
  <ConfirmDialog
    v-if="deleting && selected"
    title="Delete Scene"
    :message="`Delete scene &quot;${selected.name}&quot;?`"
    @confirm="confirmDelete"
    @cancel="deleting = false"
  />
</template>

<script setup>
import { computed, ref } from 'vue';
import Modal from './Modal.vue';
import ContextMenu from './ContextMenu.vue';
import ConfirmDialog from './ConfirmDialog.vue';
import AutoQueueSelect from './AutoQueueSelect.vue';
import ManagePresetTile from './ManagePresetTile.vue';
import { api } from '../api.js';
import { cameraName, sortedCameras, store } from '../store.js';
import { showToast } from '../toast.js';

defineEmits(['close']);

const kinds = [
  { key: 'triggerPresetId', label: 'Trigger', border: 'border-triggering' },
  { key: 'queuePresetId', label: 'Queue', border: 'border-queued' },
];

const selectedId = ref(null);
const selected = computed(() => store.scenes.find((s) => s.id === selectedId.value) ?? store.scenes[0] ?? null);
function select(id) {
  selectedId.value = id;
  picker.value = null;
}

const rows = computed(() => {
  if (!selected.value) return [];
  return sortedCameras.value
    .map((camera) => ({ camera, entry: selected.value.cameras.find((c) => c.cameraNum === camera.cameraNum) }))
    .filter((r) => r.entry);
});

const fail = (e) => alert(e.message);

function patch(row, body) {
  api.updateSceneCamera(selected.value.id, row.camera.cameraNum, body).catch(fail);
}

function slotPreset(row, kind) {
  const id = row.entry[kind.key];
  return id ? row.camera.presets.find((p) => p.id === id) ?? null : null;
}

async function addScene() {
  try {
    const sc = await api.addScene();
    select(sc.id);
  } catch (e) {
    fail(e);
  }
}

function rename(e) {
  const name = e.target.value.trim();
  if (!name || name === selected.value.name) {
    e.target.value = selected.value.name;
    return;
  }
  api.renameScene(selected.value.id, name).catch(fail);
}

async function duplicate() {
  try {
    const sc = await api.duplicateScene(selected.value.id);
    select(sc.id);
  } catch (e) {
    fail(e);
  }
}

function execute() {
  const name = selected.value.name;
  api.executeScene(selected.value.id).then(() => showToast(name)).catch(fail);
}

const menu = ref(null);
const menuItems = [
  { label: 'Duplicate', action: () => duplicate() },
  { label: 'Delete', action: () => (deleting.value = true) },
];
function openMenu(e) {
  const r = e.currentTarget.getBoundingClientRect();
  menu.value = { x: r.right - 170, y: r.bottom + 4 };
}

const deleting = ref(false);
async function confirmDelete() {
  const id = selected.value.id;
  deleting.value = false;
  picker.value = null;
  selectedId.value = null;
  api.deleteScene(id).catch(fail);
}

// preset picker: opens to the right of the modal content, closes as soon as a preset is chosen
const picker = ref(null);
const pickerCamera = computed(() => (picker.value ? store.cameras.find((c) => c.cameraNum === picker.value.cameraNum) : null));
function openPicker(row, kind) {
  picker.value = { cameraNum: row.camera.cameraNum, kind };
}
function pick(preset) {
  const { cameraNum, kind } = picker.value;
  picker.value = null;
  api.updateSceneCamera(selected.value.id, cameraNum, { [kind.key]: preset.id }).catch(fail);
}

// drag-to-reorder the scene list
const dragId = ref(null);
let dragOrder = null;
function onDragStart(sc, e) {
  dragId.value = sc.id;
  dragOrder = store.scenes.map((s) => s.id);
  e.dataTransfer.effectAllowed = 'move';
}
function onDragEnter(sc) {
  if (dragId.value === null || dragId.value === sc.id) return;
  const from = dragOrder.indexOf(dragId.value);
  const to = dragOrder.indexOf(sc.id);
  dragOrder.splice(to, 0, dragOrder.splice(from, 1)[0]);
  store.scenes = dragOrder.map((id) => store.scenes.find((s) => s.id === id));
}
function onDragEnd() {
  const order = dragOrder;
  dragId.value = null;
  dragOrder = null;
  if (order) api.reorderScenes(order).catch(fail);
}
</script>
