<template>
  <Modal title="Help" :min-width="710" @close="$emit('close')">
    <h3 class="help-section">Controls</h3>
    <div class="help-row" v-for="row in controlRows" :key="row.label">
      <div class="help-key">{{ row.label }}</div>
      <p class="help-desc">{{ row.desc }}</p>
    </div>

    <h3 class="help-section">Preset Borders</h3>
    <div class="help-row" v-for="row in rows" :key="row.label">
      <div class="preset-thumb help-swatch" :class="row.borderClass">
        <span>{{ row.label }}</span>
      </div>
      <p class="help-desc">{{ row.desc }}</p>
    </div>

    <h3 class="help-section">Groups</h3>
    <p class="help-desc help-intro">
      Groups allow presets to be automatically queued whenever a camera goes off of live. Presets must be manually
      added to that group, and only presets from the selected group will be auto-queued. There are 2 available modes.
    </p>
    <p class="help-desc help-intro"><strong>RAND</strong> - Random Mode. Presets will be auto-queued at random among the candidates.</p>
    <p class="help-desc help-intro"><strong>SEQ</strong> - Sequence Mode. The next preset after the one we just triggered will be auto-queued.</p>
    <div class="help-row">
      <div class="preset-thumb help-swatch">
        <span class="group-swatch" style="background: #2f7d4f" />
      </div>
      <p class="help-desc">The square in the top left indicates that the preset is part of the currently-selected group.</p>
    </div>
    <div class="help-row">
      <div class="preset-thumb help-swatch">
        <span class="help-taken-dot" />
      </div>
      <p class="help-desc">
        This indicates that the preset has been live at least once. In Random mode, it'll prioritize presets not yet
        taken. Gets reset when all presets in the group have been live.
      </p>
    </div>

    <h3 class="help-section">Configs</h3>
    <p class="help-desc help-intro">
      The camera settings, preset settings, and group settings are all part of the config (basically every setting
      except the ATEM host). Changing any of these will immediately update the latest auto-saved config (or create a new one if it's been 10 minutes since the last auto-save). Only the latest 50 autosaved configs are preserved. When the ShotQueue server
      starts up, it'll always load the latest config. You can also save a named config for both presistence and easier reference. Loading a named config doesn't cause the named config to be auto-saved... you have to manually save the named config to change it. 
    </p>

    <h3 class="help-section">Thumbnails</h3>
    <p class="help-desc help-intro">
      Whenever a preset becomes active, the preset's thumbnail is updated. Thumbnails don't persist across server
      restarts, and they don't persist when loading a config, so you may see missing thumbnails. You can fix this by manually triggering each preset, or you can utilize the "Gen Thumbnails" feature for convenience.
    </p>
  </Modal>
</template>

<script setup>
import Modal from './Modal.vue';

defineEmits(['close']);

const controlRows = [
  { label: 'Click', desc: "Triggers the preset, unless the camera is live, in which case it'll queue the preset." },
  { label: 'Right-Click', desc: 'Opens a context menu, where you can queue, manage group membership, reposition, rename, or delete the preset.' },
  { label: 'Ctrl-Click', desc: 'Queues the preset. Any combination of Ctrl/Alt/Shift will queue instead of trigger.' },
];

const rows = [
  { label: 'Queued', borderClass: 'border-queued', desc: 'A queued preset will be triggered the next time the camera goes off of live.' },
  { label: 'Triggering', borderClass: 'border-triggering', desc: 'A triggering preset is actively moving the camera to the preset\'s position.' },
  { label: 'Active', borderClass: 'border-active', desc: "An active preset means the camera's current position matches the preset's position." },
  { label: 'Live', borderClass: 'border-active-live', desc: 'A live preset is simply an active preset when the camera is live.' },
];
</script>
