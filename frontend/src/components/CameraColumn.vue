<template>
  <div class="camera-column" :style="{ width: unitWidth * camera.columnCount + 'px' }">
    <div class="camera-column-header">
      <div class="header-row">
        <span class="camera-name" :title="camera.name">{{ camera.name }}</span>
        <span class="tally-badge" :class="[camera.status, { hidden: camera.status === 'none' }]">{{ tallyLabel }}</span>
      </div>
      <div class="header-row">
        <select :value="camera.selectedGroupId ?? ''" @change="onGroupChange">
          <option value="">No auto-queue</option>
          <option v-for="g in camera.groups" :key="g.id" :value="g.id">{{ g.name }}</option>
        </select>
        <div class="spacer" />
        <button class="icon-btn" @click="showManage = true">⚙</button>
      </div>
    </div>

    <div class="preset-scroll">
      <div class="preset-grid" :style="{ gridTemplateColumns: `repeat(${camera.columnCount}, 1fr)` }">
        <PresetThumbnail v-for="p in camera.presets" :key="p.id" :camera="camera" :preset="p" />
        <button class="add-preset-tile" @click="showAddPreset = true">+</button>
      </div>
    </div>

    <AddPresetModal v-if="showAddPreset" :camera="camera" @close="showAddPreset = false" />
    <ManagePresetsModal v-if="showManage" :camera="camera" :unit-width="unitWidth" @close="showManage = false" />
  </div>
</template>

<script setup>
import { computed, ref } from 'vue';
import { api } from '../api.js';
import PresetThumbnail from './PresetThumbnail.vue';
import AddPresetModal from './AddPresetModal.vue';
import ManagePresetsModal from './ManagePresetsModal.vue';

const props = defineProps({ camera: Object, unitWidth: Number });

const tallyLabel = computed(() => props.camera.status.toUpperCase());

function onGroupChange(e) {
  const val = e.target.value;
  api.setSelectedGroup(props.camera.id, val ? Number(val) : null).catch((err) => alert(err.message));
}

const showAddPreset = ref(false);
const showManage = ref(false);
</script>
