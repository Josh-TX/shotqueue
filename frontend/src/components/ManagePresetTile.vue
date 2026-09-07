<template>
  <div class="preset-thumb static">
    <img v-if="!imgError" :src="imgSrc" :alt="preset.name" draggable="false" @error="imgError = true" />
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

const imgSrc = computed(() => `${props.preset.thumbnailUrl}?v=${props.preset.thumbnailVersion}`);
const imgError = ref(false);
watch(() => props.preset.thumbnailVersion, () => (imgError.value = false));
</script>
