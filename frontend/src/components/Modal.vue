<template>
  <div class="modal-backdrop" @click.self="$emit('close')">
    <div class="modal" :class="{ wide }">
      <div class="modal-header">
        <h2>{{ title }}</h2>
        <slot name="header-extra" />
        <button class="icon-btn" @click="$emit('close')">✕</button>
      </div>
      <div class="modal-body"><slot /></div>
    </div>
  </div>
</template>

<script setup>
defineProps({ title: String, wide: { type: Boolean, default: false } });
defineEmits(['close']);
</script>
