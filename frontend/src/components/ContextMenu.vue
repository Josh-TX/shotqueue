<template>
  <div class="ctx-backdrop" @click="$emit('close')" @contextmenu.prevent="$emit('close')">
    <div class="ctx-menu" :style="menuStyle" @click.stop>
      <template v-for="(item, i) in items" :key="i">
        <div v-if="item.divider" class="ctx-divider" />
        <div v-else-if="item.info" class="ctx-info">{{ item.label }}</div>
        <label
          v-else-if="item.checkbox"
          class="ctx-item ctx-checkbox-item"
          :style="item.color ? { '--group-color': item.color } : {}"
        >
          <input
            type="checkbox"
            class="g-checkbox small"
            :checked="item.checked"
            @change="item.onToggle($event.target.checked)"
          />
          {{ item.label }}
        </label>
        <button v-else class="ctx-item" :disabled="item.disabled" @click="choose(item)">
          {{ item.label }}
        </button>
      </template>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue';

const props = defineProps({ x: Number, y: Number, items: Array });
const emit = defineEmits(['close']);

const menuStyle = computed(() => {
  const maxX = window.innerWidth - 190;
  const maxY = window.innerHeight - 20;
  return {
    left: Math.min(props.x, maxX) + 'px',
    top: Math.min(props.y, maxY) + 'px',
  };
});

function choose(item) {
  if (item.disabled) return;
  item.action?.();
  emit('close');
}
</script>
