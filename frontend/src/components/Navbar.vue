<template>
  <div class="navbar">
    <span class="brand">ShotQueue</span>
    <span class="ws-dot" :class="{ on: store.wsConnected }" :title="store.wsConnected ? 'connected' : 'disconnected'" />
    <button @click="$emit('add-camera')">+ Add Camera</button>
    <div class="spacer" />
    <span v-if="genProgress" class="gen-banner">
      Generating thumbnails… ({{ genProgress.done }}/{{ genProgress.total }})
    </span>
    <button :disabled="!!genProgress" @click="$emit('gen-thumbnails')">Gen Thumbnails</button>
    <button @click="$emit('versions')">Versions</button>
    <button class="icon-btn" title="Settings" @click="$emit('settings')">⚙</button>
  </div>
</template>

<script setup>
import { computed } from 'vue';
import { store } from '../store.js';

defineEmits(['add-camera', 'settings', 'versions', 'gen-thumbnails']);

const genProgress = computed(() => {
  const active = store.cameras.filter((c) => c.generating);
  if (active.length === 0) return null;
  return {
    done: active.reduce((sum, c) => sum + (c.genDone ?? 0), 0),
    total: active.reduce((sum, c) => sum + (c.genTotal ?? 0), 0),
  };
});
</script>
