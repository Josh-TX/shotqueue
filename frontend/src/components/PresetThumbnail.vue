<template>
  <div
    class="preset-thumb"
    :class="borderClass"
    @click="onClick"
    @contextmenu.prevent="openMenu"
  >
    <img v-if="preset.thumbnailVersion != null && !imgError" :src="imgSrc" :alt="preset.name" draggable="false" @error="imgError = true" />
    <div v-else class="preset-thumb-placeholder">No thumbnail</div>
    <div class="preset-thumb-gradient" />
    <div class="preset-thumb-name">{{ preset.name }}</div>
    <span v-if="selectedGroupMember" class="group-swatch" :style="{ background: groupColor(selectedGroupMember.index) }" />
    <!-- temporary debug indicator for wasTaken, remove once the auto-queue logic is verified -->
    <div class="preset-thumb-debug" title="wasTaken">{{ preset.wasTaken ? '●' : '' }}</div>
  </div>

  <ContextMenu v-if="menu" :x="menu.x" :y="menu.y" :items="menuItems" @close="menu = null" />

  <PromptModal
    v-if="renaming"
    title="Rename Preset"
    label="Preset name"
    :initial-value="preset.name"
    submit-label="Rename"
    :on-submit="doRename"
    @close="renaming = false"
  />

  <ConfirmDialog
    v-if="deleting"
    title="Delete Preset"
    :message="`Delete preset &quot;${preset.name}&quot;? This can't be undone.`"
    @confirm="doDelete"
    @cancel="deleting = false"
  />
</template>

<script setup>
import { computed, ref, watch } from 'vue';
import { api } from '../api.js';
import { groupColor } from '../colors.js';
import ContextMenu from './ContextMenu.vue';
import PromptModal from './PromptModal.vue';
import ConfirmDialog from './ConfirmDialog.vue';

const props = defineProps({ camera: Object, preset: Object });

const isActive = computed(() => props.camera.activePresetId === props.preset.id);
const isLive = computed(() => props.camera.status === 'live');
const isTriggering = computed(() => props.camera.triggeringPresetId === props.preset.id);
const isQueued = computed(() => props.camera.queued?.presetId === props.preset.id);

const borderClass = computed(() => {
  if (isActive.value && isLive.value) return 'border-active-live';
  if (isActive.value) return 'border-active';
  if (isTriggering.value) return 'border-triggering';
  if (isQueued.value) return 'border-queued';
  return '';
});

const selectedGroupMember = computed(() => {
  const sel = props.camera.groups
    .map((g, index) => ({ ...g, index }))
    .find((g) => g.id === props.camera.selectedGroupId);
  return sel && sel.members.includes(props.preset.id) ? sel : null;
});

const imgSrc = computed(() => `/api/presets/${props.preset.id}/thumbnail?v=${props.preset.thumbnailVersion}`);
const imgError = ref(false);
watch(() => props.preset.thumbnailVersion, () => (imgError.value = false));

function trigger() {
  api.triggerPreset(props.camera.id, props.preset.id).catch((e) => alert(e.message));
}
function queue() {
  api.queuePreset(props.camera.id, props.preset.id).catch((e) => alert(e.message));
}
function unqueue() {
  api.unqueue(props.camera.id).catch((e) => alert(e.message));
}
function toggleQueue() {
  if (isActive.value || isTriggering.value) return;
  isQueued.value ? unqueue() : queue();
}

function onClick(e) {
  if (e.ctrlKey || e.shiftKey) {
    toggleQueue();
    return;
  }
  if (isActive.value) return;
  if (!isLive.value) {
    trigger();
  } else if (!isQueued.value) {
    queue();
  } else {
    unqueue();
  }
}

const menu = ref(null);
function openMenu(e) {
  menu.value = { x: e.clientX, y: e.clientY };
}
const menuItems = computed(() => [
  {
    label: isQueued.value ? 'Unqueue Preset' : 'Queue Preset',
    disabled: isActive.value || isTriggering.value,
    action: toggleQueue,
  },
  { divider: true },
  ...props.camera.groups.map((g, i) => ({
    checkbox: true,
    label: g.name,
    color: groupColor(i),
    checked: g.members.includes(props.preset.id),
    onToggle: (checked) => toggleGroup(g, checked),
  })),
  ...(props.camera.groups.length ? [{ divider: true }] : []),
  { label: 'Rename Preset', action: () => (renaming.value = true) },
  { label: 'Delete Preset', action: () => (deleting.value = true) },
]);

function toggleGroup(g, checked) {
  api.setMember(props.camera.id, g.id, props.preset.id, { inGroup: checked }).catch((e) => alert(e.message));
}

const renaming = ref(false);
async function doRename(name) {
  await api.renamePreset(props.camera.id, props.preset.id, name);
}

const deleting = ref(false);
async function doDelete() {
  await api.deletePreset(props.camera.id, props.preset.id);
  deleting.value = false;
}
</script>
