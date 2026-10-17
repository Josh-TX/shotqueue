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
          <button class="scenes-menu-item manage" @click="manage">
            <span>Manage Scenes</span>
            <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor"><path d="M19.14 12.94a7.07 7.07 0 0 0 .06-.94 7.07 7.07 0 0 0-.06-.94l2.03-1.58a.5.5 0 0 0 .12-.64l-1.92-3.32a.5.5 0 0 0-.6-.22l-2.39.96a7.3 7.3 0 0 0-1.62-.94l-.36-2.54a.5.5 0 0 0-.5-.42h-3.84a.5.5 0 0 0-.5.42l-.36 2.54a7.3 7.3 0 0 0-1.62.94l-2.39-.96a.5.5 0 0 0-.6.22L2.7 8.84a.5.5 0 0 0 .12.64l2.03 1.58a7.07 7.07 0 0 0-.06.94 7.07 7.07 0 0 0 .06.94l-2.03 1.58a.5.5 0 0 0-.12.64l1.92 3.32a.5.5 0 0 0 .6.22l2.39-.96c.5.39 1.04.71 1.62.94l.36 2.54a.5.5 0 0 0 .5.42h3.84a.5.5 0 0 0 .5-.42l.36-2.54c.58-.23 1.12-.55 1.62-.94l2.39.96a.5.5 0 0 0 .6-.22l1.92-3.32a.5.5 0 0 0-.12-.64l-2.03-1.58ZM12 15.5a3.5 3.5 0 1 1 0-7 3.5 3.5 0 0 1 0 7Z"/></svg>
          </button>
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
