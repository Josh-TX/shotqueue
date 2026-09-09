<template>
  <div class="preset-thumb static">
    <img v-if="preset.thumbnailVersion != null && !imgError" :src="imgSrc" :alt="preset.name" draggable="false" @error="imgError = true" />
    <div v-else class="preset-thumb-placeholder">No thumbnail</div>
    <slot name="overlay" />
    <div class="preset-thumb-gradient" />
    <slot name="title">
      <div class="preset-thumb-name">{{ preset.name }}</div>
    </slot>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue';

const props = defineProps({ preset: Object });

const imgSrc = computed(() => `/api/presets/${props.preset.id}/thumbnail?v=${props.preset.thumbnailVersion}`);
const imgError = ref(false);
watch(() => props.preset.thumbnailVersion, () => (imgError.value = false));
</script>
