<template>
  <div class="modal-backdrop" @contextmenu.stop @mousedown.self="onBackdropMousedown" @click.self="onBackdropClick">
    <div class="modal" :class="{ wide, tall }" :style="minWidth ? { width: minWidth + 'px' } : undefined">
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
import { ref } from 'vue';

defineProps({
  title: String,
  wide: { type: Boolean, default: false },
  tall: { type: Boolean, default: false },
  minWidth: { type: Number, default: 0 },
});
const emit = defineEmits(['close']);

const backdropMousedown = ref(false);

function onBackdropMousedown() {
  backdropMousedown.value = true;
}

function onBackdropClick() {
  if (backdropMousedown.value) emit('close');
  backdropMousedown.value = false;
}
</script>
