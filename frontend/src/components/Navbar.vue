<template>
  <div class="navbar">
    <span class="brand">ShotQueue</span>
    <span class="ws-dot" :class="{ on: store.wsConnected && store.atemConnected, warn: store.wsConnected && !store.atemConnected }" :title="statusTitle" />
    <span v-if="!store.wsConnected" class="ws-status-text">Websocket disconnected</span>
    <span v-else-if="!store.atemConnected" class="ws-status-text">ATEM disconnected</span>
    <div class="scenes-menu">
      <button class="naked-btn" @click="menuOpen = !menuOpen"><span>Scenes</span>
        <svg class="caret" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="6 9 12 15 18 9" /></svg>
      </button>
      <template v-if="menuOpen">
        <div class="scenes-menu-backdrop" @click="menuOpen = false" @contextmenu.prevent="menuOpen = false" />
        <div class="scenes-menu-list">
          <button class="scenes-menu-item manage" @click="manage">Manage Scenes</button>
          <div v-if="store.scenes.length" class="scenes-menu-divider" />
          <button v-for="sc in store.scenes" :key="sc.id" class="scenes-menu-item" @click="execute(sc)">{{ sc.name }}</button>
        </div>
      </template>
    </div>
    <div class="spacer" />
    <button @click="$emit('help')">Help</button>
    <button :disabled="anyGenerating" @click="$emit('gen-thumbnails')">Gen Thumbnails</button>
    <button @click="$emit('configs')">Configs</button>
    <button @click="$emit('settings')">Settings</button>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue';
import { api } from '../api.js';
import { store } from '../store.js';
import { showToast } from '../toast.js';

const emit = defineEmits(['settings', 'configs', 'gen-thumbnails', 'help', 'manage-scenes']);

const menuOpen = ref(false);
function manage() {
  menuOpen.value = false;
  emit('manage-scenes');
}
function execute(scene) {
  menuOpen.value = false;
  api.executeScene(scene.id).then(() => showToast(scene.name)).catch((e) => alert(e.message));
}

const statusTitle = computed(() => {
  if (!store.wsConnected) return 'websocket disconnected';
  return store.atemConnected ? 'websocket + ATEM connected' : 'websocket connected, ATEM disconnected';
});

const anyGenerating = computed(() => store.cameras.some((c) => c.generating));
</script>
