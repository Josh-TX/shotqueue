<template>
  <div class="toast-stack">
    <div v-for="t in store.toasts" :key="t.id" class="toast">{{ t.message }}</div>
  </div>
</template>

<script setup>
import { store } from '../store.js';
</script>
