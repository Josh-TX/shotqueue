<template>
  <div class="group-select" :class="{ open, selected: !!selectedColor }" :style="selectedColor ? { '--group-color': selectedColor } : {}">
    <button ref="triggerEl" type="button" class="group-select-trigger" @click="toggle">
      <span class="group-select-tiny-label">Auto Queue</span>
      <span v-if="selectedMode" class="group-select-mode-label">{{ selectedMode }}</span>
      <span class="group-select-value">
        <span class="group-select-swatch" :class="{ none: !selectedColor }" :style="selectedColor ? { background: selectedColor } : {}" />
        <span class="group-select-name">{{ selectedLabel }}</span>
      </span>
    </button>

    <div v-if="open" class="group-select-backdrop" @click="close" @contextmenu.prevent="close" />
    <div v-if="open" ref="menuEl" class="group-select-menu" :style="menuStyle">
      <button type="button" class="group-select-option" :class="{ active: !modelValue }" @click="choose(null)">
        <span class="group-select-swatch none" />
        <span class="group-select-option-name">None</span>
      </button>
      <button
        v-for="(g, i) in groups"
        :key="g.id"
        type="button"
        class="group-select-option"
        :class="{ active: modelValue === g.id }"
        @click="choose(g.id)"
      >
        <span class="group-select-swatch" :style="{ background: groupColor(i) }" />
        <span class="group-select-option-name">{{ g.name }}</span>
      </button>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, ref } from 'vue';
import { groupColor } from '../colors.js';

const props = defineProps({ modelValue: { type: Number, default: null }, groups: { type: Array, default: () => [] } });
const emit = defineEmits(['update:modelValue']);

const open = ref(false);
const triggerEl = ref(null);
const menuEl = ref(null);
const menuStyle = ref({});
async function toggle() {
  if (open.value) {
    open.value = false;
    return;
  }
  const rect = triggerEl.value.getBoundingClientRect();
  const style = { top: rect.bottom + 4 + 'px', left: rect.left + 'px', minWidth: rect.width + 'px', maxHeight: window.innerHeight - rect.bottom - 12 + 'px' };
  menuStyle.value = style;
  open.value = true;
  // flip above the trigger when it doesn't fit below
  await nextTick();
  const h = menuEl.value?.scrollHeight ?? 0;
  const below = window.innerHeight - rect.bottom - 12;
  const above = rect.top - 12;
  if (h > below && above > below) {
    menuStyle.value = { bottom: window.innerHeight - rect.top + 4 + 'px', left: style.left, minWidth: style.minWidth, maxHeight: above + 'px' };
  }
}
function close() {
  open.value = false;
}
function choose(id) {
  emit('update:modelValue', id);
  close();
}

const selectedIndex = computed(() => props.groups.findIndex((g) => g.id === props.modelValue));
const selectedLabel = computed(() => (selectedIndex.value >= 0 ? props.groups[selectedIndex.value].name : 'None'));
const selectedColor = computed(() => (selectedIndex.value >= 0 ? groupColor(selectedIndex.value) : null));
const selectedMode = computed(() => (selectedIndex.value >= 0 ? (props.groups[selectedIndex.value].isSequence ? 'SEQ' : 'RAND') : ''));
</script>
