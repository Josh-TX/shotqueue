<template>
  <div class="navbar">
    <span class="brand">ShotQueue</span>
    <span class="ws-dot" :class="{ on: store.wsConnected }" :title="store.wsConnected ? 'connected' : 'disconnected'" />
    <button @click="$emit('add-camera')">+ Add Camera</button>
    <div class="spacer" />
    <span v-if="regenProgress" class="regen-banner">
      Regenerating thumbnails… ({{ regenProgress.done }}/{{ regenProgress.total }})
    </span>
    <button @click="$emit('versions')">Versions</button>
    <button class="icon-btn" title="Settings" @click="$emit('settings')">⚙</button>
  </div>
</template>

<script setup>
import { computed } from 'vue';
import { store } from '../store.js';

defineEmits(['add-camera', 'settings', 'versions']);

const regenProgress = computed(() => {
  const active = store.cameras.filter((c) => c.regenerating);
  if (active.length === 0) return null;
  return {
    done: active.reduce((sum, c) => sum + (c.regenDone ?? 0), 0),
    total: active.reduce((sum, c) => sum + (c.regenTotal ?? 0), 0),
  };
});
</script>
