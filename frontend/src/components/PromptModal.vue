<template>
  <Modal :title="title" @close="$emit('close')">
    <div class="field">
      <label>{{ label }}</label>
      <input ref="inputEl" type="text" v-model="value" @keyup.enter="submit" />
    </div>
    <p v-if="error" class="error-text">{{ error }}</p>
    <div class="modal-actions">
      <button @click="$emit('close')">Cancel</button>
      <button class="primary" :disabled="!value.trim()" @click="submit">{{ submitLabel }}</button>
    </div>
  </Modal>
</template>

<script setup>
import { onMounted, ref } from 'vue';
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
const inputEl = ref(null);
onMounted(() => {
  inputEl.value?.focus();
  inputEl.value?.select();
});

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
