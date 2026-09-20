<template>
  <div class="ctx-backdrop" @click="$emit('close')" @contextmenu.prevent.stop="$emit('close')">
    <div ref="menuEl" class="ctx-menu" :style="menuStyle" @click.stop>
      <template v-for="(item, i) in items" :key="i">
        <div v-if="item.divider" class="ctx-divider" />
        <div v-else-if="item.info" class="ctx-info">{{ item.label }}</div>
        <div v-else-if="item.columns" class="ctx-columns">
          <div class="ctx-columns-label">{{ item.label }}</div>
          <div class="ctx-columns-grid">
            <button
              v-for="n in item.count"
              :key="n"
              class="ctx-column-btn"
              :class="{ active: n === item.value }"
              @click="item.onSelect(n)"
            >
              {{ n }}
            </button>
          </div>
        </div>
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
import { computed, onMounted, ref } from 'vue';

const props = defineProps({ x: Number, y: Number, items: Array });
const emit = defineEmits(['close']);

const menuEl = ref(null);
const menuHeight = ref(0);
onMounted(() => {
  menuHeight.value = menuEl.value?.offsetHeight ?? 0;
});

const menuStyle = computed(() => {
  const maxX = window.innerWidth - 190;
  // open upward from the click if it would overflow the bottom
  const flip = props.y + menuHeight.value > window.innerHeight - 8;
  const top = flip ? props.y - menuHeight.value : props.y;
  return {
    left: Math.min(props.x, maxX) + 'px',
    top: Math.max(8, top) + 'px',
  };
});

function choose(item) {
  if (item.disabled) return;
  item.action?.();
  emit('close');
}
</script>
