<template>
  <Modal :title="title" @close="$emit('cancel')">
    <p>{{ message }}</p>
    <div class="modal-actions">
      <button @click="$emit('cancel')">Cancel</button>
      <button class="danger" @click="$emit('confirm')">{{ confirmLabel }}</button>
    </div>
  </Modal>
</template>

<script setup>
import Modal from './Modal.vue';

defineProps({
  title: { type: String, default: 'Are you sure?' },
  message: String,
  confirmLabel: { type: String, default: 'Delete' },
});
defineEmits(['confirm', 'cancel']);
</script>
