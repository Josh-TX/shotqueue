<template>
  <div class="navbar">
    <span class="brand">ShotQueue</span>
    <span class="ws-dot" :class="{ on: store.wsConnected && store.atemConnected, warn: store.wsConnected && !store.atemConnected }" :title="statusTitle" />
    <button @click="$emit('add-camera')">+ Add Camera</button>
    <div class="spacer" />
    <button :disabled="anyGenerating" @click="$emit('gen-thumbnails')">Gen Thumbnails</button>
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

const statusTitle = computed(() => {
  if (!store.wsConnected) return 'websocket disconnected';
  return store.atemConnected ? 'websocket + ATEM connected' : 'websocket connected, ATEM disconnected';
});

const anyGenerating = computed(() => store.cameras.some((c) => c.generating));
</script>
