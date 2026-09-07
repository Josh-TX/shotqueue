<template>
  <div class="camera-column" :style="{ width: unitWidth * camera.columnCount + 'px' }">
    <div class="camera-column-header">
      <div class="header-row">
        <span class="camera-name" :title="camera.name">{{ camera.name }}</span>
        <span class="tally-badge" :class="[camera.status, { hidden: camera.status === 'none' }]">{{ tallyLabel }}</span>
      </div>
      <div class="header-row">
        <select :value="camera.selectedGroupId ?? ''" @change="onGroupChange">
          <option value="">No auto-queue</option>
          <option v-for="g in camera.groups" :key="g.id" :value="g.id">{{ g.name }}</option>
        </select>
        <div class="spacer" />
        <button class="icon-btn" @click="openMenu">⋮</button>
      </div>
    </div>

    <div class="preset-scroll">
      <div class="preset-grid" :style="{ gridTemplateColumns: `repeat(${camera.columnCount}, 1fr)` }">
        <PresetThumbnail v-for="p in camera.presets" :key="p.id" :camera="camera" :preset="p" />
        <button class="add-preset-tile" @click="showAddPreset = true">+</button>
      </div>
    </div>

    <ContextMenu v-if="menu" :x="menu.x" :y="menu.y" :items="menuItems" @close="menu = null" />

    <AddPresetModal v-if="showAddPreset" :camera="camera" @close="showAddPreset = false" />
    <ManageGroupsModal v-if="showGroups" :camera="camera" @close="showGroups = false" />

    <PromptModal
      v-if="renaming"
      title="Rename Camera"
      label="Camera name"
      :initial-value="camera.name"
      submit-label="Rename"
      :on-submit="doRename"
      @close="renaming = false"
    />
    <ConfirmDialog
      v-if="deleting"
      title="Delete Camera"
      :message="`Delete &quot;${camera.name}&quot; and all its presets and groups?`"
      @confirm="doDelete"
      @cancel="deleting = false"
    />
  </div>
</template>

<script setup>
import { computed, ref } from 'vue';
import { api } from '../api.js';
import PresetThumbnail from './PresetThumbnail.vue';
import ContextMenu from './ContextMenu.vue';
import PromptModal from './PromptModal.vue';
import ConfirmDialog from './ConfirmDialog.vue';
import AddPresetModal from './AddPresetModal.vue';
import ManageGroupsModal from './ManageGroupsModal.vue';

const props = defineProps({ camera: Object, unitWidth: Number });

const tallyLabel = computed(() => props.camera.status.toUpperCase());

function onGroupChange(e) {
  const val = e.target.value;
  api.setSelectedGroup(props.camera.id, val ? Number(val) : null).catch((err) => alert(err.message));
}

const menu = ref(null);
function openMenu(e) {
  menu.value = { x: e.clientX, y: e.clientY };
}
const menuItems = computed(() => [
  { label: 'Manage Groups', action: () => (showGroups.value = true) },
  { label: 'Change Column Count', action: changeColumnCount },
  { divider: true },
  { label: 'Rename Camera', action: () => (renaming.value = true) },
  { label: 'Delete Camera', action: () => (deleting.value = true) },
]);

function changeColumnCount() {
  const input = window.prompt('Column count (1-5)', String(props.camera.columnCount));
  if (input === null) return;
  const n = Number(input);
  if (!Number.isInteger(n) || n < 1 || n > 5) return;
  api.updateCamera(props.camera.id, { columnCount: n }).catch((err) => alert(err.message));
}

const showAddPreset = ref(false);
const showGroups = ref(false);

const renaming = ref(false);
async function doRename(name) {
  await api.updateCamera(props.camera.id, { name });
}

const deleting = ref(false);
async function doDelete() {
  await api.deleteCamera(props.camera.id);
  deleting.value = false;
}
</script>
