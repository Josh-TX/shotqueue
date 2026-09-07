<template>
  <Modal :title="title" @close="$emit('close')">
    <div class="field">
      <label>{{ label }}</label>
      <input type="text" v-model="value" @keyup.enter="submit" autofocus />
    </div>
    <p v-if="error" class="error-text">{{ error }}</p>
    <div class="modal-actions">
      <button @click="$emit('close')">Cancel</button>
      <button class="primary" :disabled="!value.trim()" @click="submit">{{ submitLabel }}</button>
    </div>
  </Modal>
</template>

<script setup>
import { ref } from 'vue';
import Modal from './Modal.vue';

const props = defineProps({
  title: String,
  label: String,
  initialValue: { type: String, default: '' },
  submitLabel: { type: String, default: 'Save' },
  onSubmit: Function,
});
const emit = defineEmits(['close']);

const value = ref(props.initialValue);
const error = ref('');

async function submit() {
  if (!value.value.trim()) return;
  try {
    await props.onSubmit(value.value.trim());
    emit('close');
  } catch (e) {
    error.value = e.message;
  }
}
</script>
