<template>
  <div
    class="preset-thumb"
    :class="borderClass"
    @click="onClick"
    @contextmenu.prevent="openMenu"
  >
    <img v-if="!imgError" :src="imgSrc" :alt="preset.name" draggable="false" @error="imgError = true" />
    <div class="preset-thumb-groups">
      <span
        v-for="g in memberGroups"
        :key="g.id"
        class="group-dot"
        :class="{ big: isQueued && queuedAuto && g.id === camera.selectedGroupId }"
        :style="{ background: GROUP_COLOR_HEX[g.color] }"
      />
    </div>
    <div class="preset-thumb-gradient" />
    <div class="preset-thumb-name">{{ preset.name }}</div>
    <div class="preset-thumb-scene-count" title="Scene takes">{{ preset.scene.takenCount }}</div>
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
import { GROUP_COLOR_HEX } from '../colors.js';
import { formatDuration } from '../utils.js';
import ContextMenu from './ContextMenu.vue';
import PromptModal from './PromptModal.vue';
import ConfirmDialog from './ConfirmDialog.vue';

const props = defineProps({ camera: Object, preset: Object });

const isActive = computed(() => props.camera.activePresetId === props.preset.id);
const isLive = computed(() => props.camera.status === 'live');
const isTriggering = computed(() => props.camera.triggeringPresetId === props.preset.id);
const isQueued = computed(() => props.camera.queued?.presetId === props.preset.id);
const queuedAuto = computed(() => isQueued.value && props.camera.queued?.origin === 'auto');

const borderClass = computed(() => {
  if (isActive.value && isLive.value) return 'border-active-live';
  if (isActive.value) return 'border-active';
  if (isTriggering.value) return 'border-triggering';
  if (isQueued.value) return 'border-queued';
  return '';
});

const memberGroups = computed(() =>
  props.camera.groups.filter((g) => g.members.some((m) => m.presetId === props.preset.id))
);

const imgSrc = computed(() => `${props.preset.thumbnailUrl}?v=${props.preset.thumbnailVersion}`);
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
  {
    info: true,
    label: `Scene: ${props.preset.scene.takenCount} takes\n${formatDuration(props.preset.scene.liveTimeMs)} live`,
  },
  {
    info: true,
    label: `Show: ${props.preset.show.takenCount} takes\n${formatDuration(props.preset.show.liveTimeMs)} live`,
  },
  { divider: true },
  { label: 'Rename Preset', action: () => (renaming.value = true) },
  { label: 'Delete Preset', action: () => (deleting.value = true) },
]);

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
