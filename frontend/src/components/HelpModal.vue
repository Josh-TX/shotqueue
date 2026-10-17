<template>
  <Modal title="Help" :min-width="710" @close="$emit('close')">
    <h3 class="help-section">Controls</h3>
    <div class="help-row" v-for="row in controlRows" :key="row.label">
      <div class="help-key">{{ row.label }}</div>
      <p class="help-desc">{{ row.desc }}</p>
    </div>

    <h3 class="help-section">Preset Borders</h3>
    <div class="help-row" v-for="row in rows" :key="row.label">
      <div class="preset-thumb static help-swatch" :class="row.borderClass">
        <span>{{ row.label }}</span>
      </div>
      <p class="help-desc">{{ row.desc }}</p>
    </div>

    <h3 class="help-section">Auto Queue</h3>
    <p class="help-desc help-intro">
      You can assign presets to a group, and have that group be auto-queued whenever a camera goes off of live. You can
      use "sequential mode" and presets will be queued in order, or "random" mode and preset will be queued in a random
      order.
    </p>
    <p class="help-desc help-intro">
      You can also auto-queue multiple groups instead of just 1. In alternate mode, it'll alternate between the 2 (or
      more) groups, even if the groups have a different number of presets assigned. RAND/SEQ mode still affects how
      presets are selected within each single group. In union mode, it's effectively the same as having a single group
      that contains all presets among the selected groups.
    </p>
    <div class="help-row">
      <div class="preset-thumb static help-swatch">
        <div class="group-swatches"><span class="group-swatch" style="background: #2f7d4f" /></div>
      </div>
      <p class="help-desc">The square in the top left indicates that the preset is part an auto-queued group.</p>
    </div>
    <div class="help-row">
      <div class="preset-thumb static help-swatch">
        <span class="help-taken-dot" />
      </div>
      <p class="help-desc">
        This indicates that the preset has been live at least once. In Random mode, it'll prioritize presets not yet
        taken. Gets reset when all presets in the group have been live.
      </p>
    </div>

    <h3 class="help-section">Scenes</h3>
    <p class="help-desc help-intro" style="margin-bottom: 6px">
      Scenes are a way to quickly execute several actions at once. There is no concept of a "selected scene". Scenes can
      only do 4 types of actions:
    </p>
    <ul class="help-desc help-intro help-list">
      <li>set camera visibility</li>
      <li>set auto-queue</li>
      <li>trigger a preset</li>
      <li>queue a preset</li>
    </ul>
    <p class="help-desc help-intro">
      Triggering a scene will never move a live camera. If it tries to, the preset will be queued instead of triggered.
      Scenes have zero effect on which presets exist, the position/focus of a preset, or the group assignments.
    </p>

    <h3 class="help-section">Focus</h3>
    <p class="help-desc help-intro">
      Whenever a preset finishes triggering and becomes active, shotqueue will attempt to one-time focus that camera.
      You can right-click any active preset and "Lock" the focus. This stores the current focus settings to the preset,
      and whenever the preset is triggered, it'll update the focus to the locked value. Presets with a locked focus won't perform the
      one-time focus upon becoming active.
    </p>
    <div class="help-row">
      <div class="preset-thumb static help-swatch">
        <span class="focus-lock-badge">F</span>
      </div>
      <p class="help-desc">This indicates the preset has a focus locked.</p>
    </div>
    <div class="help-row">
      <div class="preset-thumb static help-swatch">
        <span class="focus-lock-badge">A</span>
      </div>
      <p class="help-desc">
        This indicates that the camera is internally in auto-focus mode, and therefore any locked focuses will be
        ignored by the camera. Shotqueue doesn't alter auto-focus mode, and it must be managed by an external PTZ
        controller.
      </p>
    </div>

    <h3 class="help-section">Configs</h3>
    <p class="help-desc help-intro">
      The camera visibility, preset settings, and group settings are all part of the config (the camera connection
      settings and ATEM settings are not). Changing any of these will immediately create a new auto-saved config. The 2 latest autosaves are always kept; older ones are thinned so they're at least 5 minutes apart. Only the latest 50 autosaved configs are preserved. When the ShotQueue server
      starts up, it'll always load the latest config. You can also save a named config for both presistence and easier reference. Loading a named config doesn't cause the named config to be auto-saved... you have to manually save the named config to change it.
    </p>
    <p v-if="configPath" class="help-desc help-intro">Configs are stored at <code>{{ configPath }}</code></p>

    <h3 class="help-section">Thumbnails</h3>
    <p class="help-desc help-intro">
      Whenever a preset becomes active, the preset's thumbnail is updated. Thumbnails don't persist across server
      restarts, and loaded configs might contain new presets, so you may see missing thumbnails. You can fix this by manually triggering each preset, or you can utilize the "Gen Thumbnails" feature for convenience.
    </p>
  </Modal>
</template>

<script setup>
import { ref } from 'vue';
import { api } from '../api.js';
import Modal from './Modal.vue';

defineEmits(['close']);

const configPath = ref('');
api
  .getSettings()
  .then((s) => (configPath.value = s.configPath ?? ''))
  .catch(() => {});

const controlRows = [
  { label: 'Click', desc: "Triggers the preset, unless the camera is live, in which case it'll queue the preset." },
  { label: 'RightClick', desc: 'Opens a context menu, where you can queue, manage group membership, reposition, rename, or delete the preset. On Mac, Ctrl-Click and a two-finger tap also open it.' },
  { label: 'Ctrl+Click', desc: 'Queues the preset. Any combination of Shift/Alt/Ctrl/Cmd will queue instead of trigger.' },
];

const rows = [
  { label: 'Queued', borderClass: 'border-queued', desc: 'A queued preset will be triggered the next time the camera goes off of live.' },
  { label: 'Triggering', borderClass: 'border-triggering', desc: 'A triggering preset is actively moving the camera to the preset\'s position.' },
  { label: 'Active', borderClass: 'border-active', desc: "An active preset means the camera's current position matches the preset's position." },
  { label: 'Live', borderClass: 'border-active-live', desc: 'A live preset is simply an active preset when the camera is live.' },
];
</script>
