<template>
  <div class="navbar">
    <span class="brand">PTZ Control</span>
    <span class="ws-dot" :class="{ on: store.wsConnected }" :title="store.wsConnected ? 'connected' : 'disconnected'" />
    <button @click="$emit('add-camera')">+ Add Camera</button>
    <div class="spacer" />
    <span v-if="regenProgress" class="regen-banner">
      Regenerating thumbnails… ({{ regenProgress.done }}/{{ regenProgress.total }})
    </span>
    <button @click="resetScene">Reset Scene</button>
    <button @click="resetShow">Reset Show</button>
    <button @click="$emit('versions')">Versions</button>
    <button class="icon-btn" title="Settings" @click="$emit('settings')">⚙</button>
  </div>
</template>

<script setup>
import { computed } from 'vue';
import { store } from '../store.js';
import { api } from '../api.js';

defineEmits(['add-camera', 'settings', 'versions']);

const regenProgress = computed(() => {
  const active = store.cameras.filter((c) => c.regenerating);
  if (active.length === 0) return null;
  return {
    done: active.reduce((sum, c) => sum + (c.regenDone ?? 0), 0),
    total: active.reduce((sum, c) => sum + (c.regenTotal ?? 0), 0),
  };
});

function resetShow() {
  if (confirm('Reset show metrics for all cameras? This also resets scene metrics.')) {
    api.resetShow().catch((e) => alert(e.message));
  }
}
function resetScene() {
  if (confirm('Reset scene metrics for all cameras?')) {
    api.resetScene().catch((e) => alert(e.message));
  }
}
</script>
