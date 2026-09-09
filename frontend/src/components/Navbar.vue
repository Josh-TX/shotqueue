<template>
  <div class="navbar">
    <span class="brand">ShotQueue</span>
    <span class="ws-dot" :class="{ on: store.wsConnected && store.atemConnected, warn: store.wsConnected && !store.atemConnected }" :title="statusTitle" />
    <span v-if="!store.wsConnected" class="ws-status-text">Websocket disconnected</span>
    <span v-else-if="!store.atemConnected" class="ws-status-text">ATEM disconnected</span>
    <div class="spacer" />
    <button @click="$emit('help')">Help</button>
    <button :disabled="anyGenerating" @click="$emit('gen-thumbnails')">Gen Thumbnails</button>
    <button @click="$emit('configs')">Configs</button>
    <button @click="$emit('settings')">Settings</button>
  </div>
</template>

<script setup>
import { computed } from 'vue';
import { store } from '../store.js';

defineEmits(['settings', 'configs', 'gen-thumbnails', 'help']);

const statusTitle = computed(() => {
  if (!store.wsConnected) return 'websocket disconnected';
  return store.atemConnected ? 'websocket + ATEM connected' : 'websocket connected, ATEM disconnected';
});

const anyGenerating = computed(() => store.cameras.some((c) => c.generating));
</script>
