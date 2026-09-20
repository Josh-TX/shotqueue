<template>
  <div class="config-preview-row" ref="rowEl">
    <div v-for="(c, i) in shownCameras" :key="i" class="config-preview-column">
      <div class="config-preview-name" :title="cameraName(c)">{{ cameraName(c) }}</div>
      <div
        class="config-preview-grid"
        :style="{ gridTemplateColumns: `repeat(${c.columnCount}, 1fr)`, width: columnWidth(c) + 'px' }"
      >
        <div v-for="(p, pIdx) in c.presets" :key="pIdx" class="config-preview-rect" :title="p.name">
          <div v-if="groupIndexesFor(c, pIdx).length" class="config-preview-swatches">
            <span
              v-for="gi in groupIndexesFor(c, pIdx)"
              :key="gi"
              class="config-preview-swatch"
              :style="{ background: groupColor(gi) }"
            />
          </div>
          <div class="config-preview-rect-name">{{ p.name }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue';
import { groupColor } from '../colors.js';
import { cameraName } from '../store.js';

const props = defineProps({ cameras: { type: Array, default: () => [] } });

// hidden cameras keep their presets in the config but aren't part of what's shown on load
const shownCameras = computed(() => props.cameras.filter((c) => !c.isHidden));

const GAP = 4;
const MIN_UNIT_WIDTH = 16;
// must match .config-preview-column + .config-preview-column border-left + padding-left in style.css
const CAMERA_OVERHEAD = 5;

const rowEl = ref(null);
const containerWidth = ref(0);
onMounted(() => {
  containerWidth.value = rowEl.value?.clientWidth ?? 0;
});

const totalColumns = computed(() => Math.max(1, shownCameras.value.reduce((sum, c) => sum + c.columnCount, 0)));
const cameraCount = computed(() => Math.max(1, shownCameras.value.length));

const unitWidth = computed(() => {
  const raw =
    (containerWidth.value - GAP * (totalColumns.value - 1) - CAMERA_OVERHEAD * (cameraCount.value - 1)) /
    totalColumns.value;
  return Math.max(MIN_UNIT_WIDTH, raw);
});

function columnWidth(c) {
  return unitWidth.value * c.columnCount + GAP * (c.columnCount - 1);
}

function groupIndexesFor(c, presetIdx) {
  return (c.groups ?? []).map((g, i) => (g.members.includes(presetIdx) ? i : -1)).filter((i) => i !== -1);
}
</script>
