<template>
  <div class="group-select" :class="{ open, selected: selectedGroups.length > 0, multi: selectedGroups.length > 1 }" :style="rootStyle">
    <button ref="triggerEl" type="button" class="group-select-trigger" @click="toggle($event)" @contextmenu.prevent.stop="toggle($event)">
      <span class="group-select-tiny-label">Auto Queue</span>
      <span v-if="selectedMode" class="group-select-mode-label">{{ selectedMode }}</span>
      <span v-if="selectedGroups.length === 2" class="group-select-value">
        <template v-for="(g, i) in selectedGroups" :key="g.id">
          <span v-if="i" class="group-select-pipe">|</span>
          <span class="group-select-swatch" :style="{ background: g.color }" />
          <span class="group-select-name fair">{{ g.name }}</span>
        </template>
      </span>
      <span v-else-if="selectedGroups.length > 2" class="group-select-value">
        <span v-for="g in selectedGroups" :key="g.id" class="group-select-swatch" :style="{ background: g.color }" />
        <span class="group-select-name">({{ selectedGroups.length }} groups selected)</span>
      </span>
      <span v-else class="group-select-value">
        <span class="group-select-swatch" :class="{ none: !selectedGroups.length }" :style="selectedGroups.length ? { background: selectedGroups[0].color } : {}" />
        <span class="group-select-name">{{ selectedGroups.length ? selectedGroups[0].name : 'None' }}</span>
      </span>
    </button>

    <div v-if="open" class="group-select-backdrop" @click="close" @contextmenu.prevent.stop="close" />
    <div v-if="open" ref="menuEl" class="group-select-menu" :style="menuStyle" @contextmenu.prevent.stop>
      <button type="button" class="group-select-option" :class="{ active: !multi && !modelValue && !selectedMode }" @click="choose(null)">
        <span class="group-select-swatch none" />
        <span class="group-select-option-name">None</span>
      </button>
      <button
        v-for="g in coloredGroups"
        :key="g.id"
        type="button"
        class="group-select-option"
        :class="{ active: !multi && modelValue === g.id }"
        @click="multi ? toggleChecked(g.id) : choose(g.id)"
      >
        <input v-if="multi" type="checkbox" class="g-checkbox small group-select-check" tabindex="-1" :checked="checkedIds.includes(g.id)" :style="{ '--group-color': g.color }" />
        <span v-else class="group-select-swatch" :style="{ background: g.color }" />
        <span class="group-select-option-name">{{ g.name }}</span>
      </button>
      <div class="group-select-divider" />
      <button type="button" class="group-select-option" :class="{ active: multi && multiMode === 'alternate' }" @click="enterMulti('alternate')">
        <span class="group-select-option-name">Multiple (alternate)</span>
      </button>
      <button type="button" class="group-select-option" :class="{ active: multi && multiMode === 'union' }" @click="enterMulti('union')">
        <span class="group-select-option-name">Multiple (union)</span>
      </button>
      <button v-if="multi" type="button" class="group-select-option" @click="cancelMulti">
        <span class="group-select-option-name">Cancel multiple</span>
      </button>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue';
import { groupColor } from '../colors.js';

// Emits `change` with { groupId, groupIds, mode }: none = all empty, single = groupId, multi = mode ('alternate' | 'union') + groupIds.
const props = defineProps({
  modelValue: { type: Number, default: null },
  selectedGroupIds: { type: Array, default: () => [] },
  selectedMode: { type: String, default: '' },
  lastMultipleMode: { type: String, default: 'alternate' },
  groups: { type: Array, default: () => [] },
  isSequence: Boolean,
});
const emit = defineEmits(['change']);

const open = ref(false);
const multi = ref(false); // dropdown is showing checkboxes
const multiMode = ref('alternate');
const checkedIds = ref([]);
const triggerEl = ref(null);
const menuEl = ref(null);
const menuStyle = ref({});
async function toggle(e) {
  if (open.value) {
    open.value = false;
    return;
  }
  multi.value = !!props.selectedMode;
  if (multi.value) {
    multiMode.value = props.selectedMode;
    checkedIds.value = [...props.selectedGroupIds];
  }
  const rect = triggerEl.value.getBoundingClientRect();
  const style = { top: rect.bottom + 2 + 'px', left: rect.left + 'px', minWidth: rect.width + 'px', maxHeight: window.innerHeight - rect.bottom - 12 + 'px' };
  menuStyle.value = style;
  open.value = true;
  if (e && hasMod(e)) modEntered(); // opened while holding a modifier
  // flip above the trigger when it doesn't fit below
  await nextTick();
  const h = menuEl.value?.scrollHeight ?? 0;
  const below = window.innerHeight - rect.bottom - 12;
  const above = rect.top - 12;
  if (h > below && above > below) {
    menuStyle.value = { bottom: window.innerHeight - rect.top + 2 + 'px', left: style.left, minWidth: style.minWidth, maxHeight: above + 'px' };
  }
}
const hasMod = (e) => e.ctrlKey || e.shiftKey || e.altKey || e.metaKey;
let enteredByMod = false;
function modEntered() {
  if (multi.value) return; // already multi; leave as is
  enterMulti(props.lastMultipleMode || 'alternate');
  enteredByMod = true;
}
function onKeyDown(e) {
  if (hasMod(e)) modEntered();
}
function onKeyUp(e) {
  if (!hasMod(e) && enteredByMod) {
    enteredByMod = false;
    if (!checkedIds.value.length) cancelMulti();
  }
}
watch(open, (v) => {
  if (v) {
    window.addEventListener('keydown', onKeyDown);
    window.addEventListener('keyup', onKeyUp);
  } else {
    window.removeEventListener('keydown', onKeyDown);
    window.removeEventListener('keyup', onKeyUp);
    enteredByMod = false;
  }
});
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeyDown);
  window.removeEventListener('keyup', onKeyUp);
});
function close() {
  open.value = false;
}
function choose(id) {
  multi.value = false;
  emit('change', { groupId: id, groupIds: [], mode: '' });
  close();
}
function emitMulti() {
  emit('change', { groupId: null, groupIds: checkedIds.value, mode: checkedIds.value.length ? multiMode.value : '' });
}
function cancelMulti() {
  multi.value = false;
  checkedIds.value = [];
  emit('change', { groupId: null, groupIds: [], mode: '' });
}
function enterMulti(mode) {
  if (!multi.value) {
    // entering multi mode starts from none
    multi.value = true;
    checkedIds.value = [];
  }
  multiMode.value = mode;
  emitMulti();
}
function toggleChecked(id) {
  checkedIds.value = checkedIds.value.includes(id) ? checkedIds.value.filter((x) => x !== id) : [...checkedIds.value, id];
  emitMulti();
}

const coloredGroups = computed(() => props.groups.map((g, i) => ({ ...g, color: groupColor(g, i) })));
const selectedGroups = computed(() => {
  if (props.selectedMode) return coloredGroups.value.filter((g) => props.selectedGroupIds.includes(g.id));
  const g = coloredGroups.value.find((g) => g.id === props.modelValue);
  return g ? [g] : [];
});
const selectedMode = computed(() => {
  if (props.selectedMode === 'alternate') return 'ALTER';
  if (props.selectedMode === 'union') return 'UNION';
  return selectedGroups.value.length ? (props.isSequence ? 'SEQ' : 'RAND') : '';
});
const rootStyle = computed(() => {
  const colors = selectedGroups.value.map((g) => g.color);
  if (!colors.length) return {};
  const style = { '--group-color': colors[0] };
  if (colors.length > 1) {
    style['--group-border'] = `linear-gradient(90deg, ${colors.join(', ')})`;
    style['--group-fill'] = `linear-gradient(90deg, ${colors.map((c) => `color-mix(in srgb, ${c} 22%, var(--panel2))`).join(', ')})`;
  }
  return style;
});
</script>
