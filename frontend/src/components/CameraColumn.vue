<template>
  <div class="camera-column">
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

    <div v-if="camera.presets.length === 0" class="no-presets">no presets</div>
    <div v-else class="preset-scroll">
      <PresetThumbnail v-for="p in camera.presets" :key="p.id" :camera="camera" :preset="p" />
    </div>

    <ContextMenu v-if="menu" :x="menu.x" :y="menu.y" :items="menuItems" @close="menu = null" />

    <CameraInfoModal v-if="showInfo" :camera="camera" @close="showInfo = false" />
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
import CameraInfoModal from './CameraInfoModal.vue';
import AddPresetModal from './AddPresetModal.vue';
import ManageGroupsModal from './ManageGroupsModal.vue';

const props = defineProps({ camera: Object });

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
  { label: 'Info', action: () => (showInfo.value = true) },
  { label: 'Add Preset', action: () => (showAddPreset.value = true) },
  { label: 'Manage Groups', action: () => (showGroups.value = true) },
  { divider: true },
  { label: 'Rename Camera', action: () => (renaming.value = true) },
  { label: 'Delete Camera', action: () => (deleting.value = true) },
]);

const showInfo = ref(false);
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
