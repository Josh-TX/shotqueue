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
    <button class="icon-btn gear-btn" title="Settings" @click="$emit('settings')">
      <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor"><path d="M19.14 12.94a7.07 7.07 0 0 0 .06-.94 7.07 7.07 0 0 0-.06-.94l2.03-1.58a.5.5 0 0 0 .12-.64l-1.92-3.32a.5.5 0 0 0-.6-.22l-2.39.96a7.3 7.3 0 0 0-1.62-.94l-.36-2.54a.5.5 0 0 0-.5-.42h-3.84a.5.5 0 0 0-.5.42l-.36 2.54a7.3 7.3 0 0 0-1.62.94l-2.39-.96a.5.5 0 0 0-.6.22L2.7 8.84a.5.5 0 0 0 .12.64l2.03 1.58a7.07 7.07 0 0 0-.06.94 7.07 7.07 0 0 0 .06.94l-2.03 1.58a.5.5 0 0 0-.12.64l1.92 3.32a.5.5 0 0 0 .6.22l2.39-.96c.5.39 1.04.71 1.62.94l.36 2.54a.5.5 0 0 0 .5.42h3.84a.5.5 0 0 0 .5-.42l.36-2.54c.58-.23 1.12-.55 1.62-.94l2.39.96a.5.5 0 0 0 .6-.22l1.92-3.32a.5.5 0 0 0-.12-.64l-2.03-1.58ZM12 15.5a3.5 3.5 0 1 1 0-7 3.5 3.5 0 0 1 0 7Z"/></svg>
    </button>
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
