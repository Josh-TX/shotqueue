<template>
  <div class="navbar">
    <span class="brand">PTZ Control</span>
    <span class="ws-dot" :class="{ on: store.wsConnected }" :title="store.wsConnected ? 'connected' : 'disconnected'" />
    <button @click="$emit('add-camera')">+ Add Camera</button>
    <div class="spacer" />
    <button @click="resetScene">Reset Scene</button>
    <button @click="resetShow">Reset Show</button>
    <button class="icon-btn" title="Settings" @click="$emit('settings')">⚙</button>
  </div>
</template>

<script setup>
import { store } from '../store.js';
import { api } from '../api.js';

defineEmits(['add-camera', 'settings']);

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
