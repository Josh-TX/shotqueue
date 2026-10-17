<template>
  <Modal :title="cameraLabel" tall :min-width="modalWidth" @close="$emit('close')">
    <div class="manage-shell">
      <div class="manage-sidebar">
        <button class="manage-tab" :class="{ active: tab === 'general' }" @click="tab = 'general'">General</button>
        <button class="manage-tab" :class="{ active: tab === 'groups' }" @click="tab = 'groups'">Groups</button>
      </div>

      <div class="manage-content">
        <template v-if="tab === 'general'">
          <p class="help-text">Connection info is managed on the <a href="#" class="settings-link" @click.prevent="openSettings">Settings page</a>.</p>
          <div class="manage-info-row">
            <span>Thumbnail Columns</span>
            <select :value="camera.columnCount" @change="setColumnCount($event.target.value)">
              <option v-for="n in 6" :key="n" :value="n">{{ n }}</option>
            </select>
          </div>

          <div class="general-actions">
            <button @click="openAddPreset">+ Add Preset</button>
          </div>

          <div class="manage-preset-grid" :style="gridStyle">
            <ManagePresetTile v-for="p in camera.presets" :key="p.id" :preset="p">
              <template #overlay>
                <div class="thumb-overlay-btns">
                  <button type="button" class="thumb-overlay-btn thumb-overlay-btn-text" @click.stop="$emit('reposition', p)">
                    Reposition
                  </button>
                  <button type="button" class="thumb-overlay-btn" title="Delete" @click.stop="deletePreset(p)">
                    <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <polyline points="3 6 5 6 21 6" />
                      <path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" />
                      <path d="M10 11v6" />
                      <path d="M14 11v6" />
                      <path d="M9 6V4a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2v2" />
                    </svg>
                  </button>
                </div>
              </template>
              <template #title>
                <input
                  type="text"
                  class="preset-thumb-name-input"
                  :value="p.name"
                  @click.stop
                  @blur="renamePreset(p, $event.target.value)"
                  @keyup.enter="$event.target.blur()"
                />
              </template>
            </ManagePresetTile>
            <div v-if="camera.presets.length === 0" class="no-presets">No presets yet</div>
          </div>
        </template>

        <template v-else-if="tab === 'groups'">
          <p class="help-text">Groups allow you to have presets auto-queued when you go off of live.</p>
          <div class="manage-info-row">
            <span>Selection Mode</span>
            <div class="manage-radio-group">
              <label><input type="radio" :checked="!camera.isSequence" @change="setSequenceMode('random')" /> Random</label>
              <label><input type="radio" :checked="camera.isSequence" @change="setSequenceMode('sequence')" /> Sequence</label>
            </div>
          </div>

          <div class="manage-group-names">
            <template v-for="(g, i) in camera.groups" :key="g.id">
              <div class="manage-group-name-row">
                <div class="manage-group-color-wrap">
                  <button
                    type="button"
                    class="manage-group-swatch-btn"
                    title="Change color"
                    @click="colorPickerId = colorPickerId === g.id ? null : g.id"
                  >
                    <span class="manage-group-swatch" :style="{ background: groupColor(g, i) }" />
                  </button>
                  <template v-if="colorPickerId === g.id">
                    <div class="manage-group-colors-backdrop" @click="colorPickerId = null" />
                    <div class="manage-group-colors">
                      <button
                        v-for="c in GROUP_COLORS"
                        :key="c"
                        type="button"
                        class="manage-group-color-opt"
                        :class="{ active: groupColor(g, i) === c }"
                        :style="{ background: c }"
                        @click="setGroupColor(g, c)"
                      />
                    </div>
                  </template>
                </div>
                <input
                  type="text"
                  :value="g.name"
                  @blur="renameGroup(g, $event.target.value)"
                  @keyup.enter="$event.target.blur()"
                />
                <button type="button" class="naked-btn" title="Delete group" @click="deleteGroup(g)">&times;</button>
              </div>
            </template>
          </div>

          <div class="general-actions group-add-row">
            <button v-if="camera.groups.length < MAX_GROUPS" @click="addGroup">+ Add Group</button>
            <span v-else class="help-text">(max {{ MAX_GROUPS }} groups)</span>
          </div>

          <div class="manage-preset-grid" :style="gridStyle">
            <ManagePresetTile v-for="p in camera.presets" :key="p.id" :preset="p">
              <template #overlay>
                <div class="manage-group-checks">
                  <input
                    v-for="(g, i) in camera.groups"
                    :key="g.id"
                    type="checkbox"
                    class="g-checkbox"
                    :style="{ '--group-color': groupColor(g, i) }"
                    :title="g.name"
                    :checked="g.members.includes(p.id)"
                    @change="toggleMember(g, p, $event.target.checked)"
                  />
                </div>
              </template>
            </ManagePresetTile>
            <div v-if="camera.presets.length === 0" class="no-presets">No presets yet</div>
          </div>
        </template>
      </div>
    </div>
  </Modal>
</template>

<script setup>
import { computed, ref } from 'vue';
import Modal from './Modal.vue';
import ManagePresetTile from './ManagePresetTile.vue';
import { api } from '../api.js';
import { cameraName } from '../store.js';
import { groupColor, GROUP_COLORS, MAX_GROUPS } from '../colors.js';

const props = defineProps({ camera: Object, unitWidth: Number, initialTab: { type: String, default: 'general' } });
const emit = defineEmits(['close', 'add-preset', 'reposition', 'open-settings']);

const tab = ref(props.initialTab);
const colorPickerId = ref(null);

const MIN_TILE_WIDTH = 160;
const tileWidth = computed(() => Math.max(MIN_TILE_WIDTH, props.unitWidth));

const gridStyle = computed(() => ({
  gridTemplateColumns: `repeat(${props.camera.columnCount}, ${tileWidth.value}px)`,
}));

const manageShellOverhead = 230; // sidebar + gaps + modal padding + scrollbar
const modalWidth = computed(() => {
  const gridWidth = props.camera.columnCount * tileWidth.value + (props.camera.columnCount - 1) * 10;
  return Math.max(620, gridWidth + manageShellOverhead);
});

const cameraLabel = computed(() => cameraName(props.camera));

function renamePreset(p, name) {
  api.renamePreset(props.camera.cameraNum, p.id, name).catch((e) => alert(e.message));
}

function deletePreset(p) {
  if (!confirm(`Delete preset "${p.name}"?`)) return;
  api.deletePreset(props.camera.cameraNum, p.id).catch((e) => alert(e.message));
}

function openAddPreset() {
  emit('add-preset');
}

function openSettings() {
  emit('close');
  emit('open-settings');
}

function addGroup() {
  api.addGroup(props.camera.cameraNum).catch((e) => alert(e.message));
}
function deleteGroup(g) {
  if (!confirm(`Delete group "${g.name}"?`)) return;
  api.deleteGroup(props.camera.cameraNum, g.id).catch((e) => alert(e.message));
}
function setGroupColor(g, color) {
  colorPickerId.value = null;
  api.updateGroup(props.camera.cameraNum, g.id, { color }).catch((e) => alert(e.message));
}
function renameGroup(g, name) {
  if (!name.trim()) return;
  api.updateGroup(props.camera.cameraNum, g.id, { name: name.trim() }).catch((e) => alert(e.message));
}
function setSequenceMode(value) {
  api.setSequenceMode(props.camera.cameraNum, value === 'sequence').catch((e) => alert(e.message));
}
function toggleMember(g, p, checked) {
  api.setMember(props.camera.cameraNum, g.id, p.id, { inGroup: checked }).catch((e) => alert(e.message));
}

function setColumnCount(value) {
  api.setColumnCount(props.camera.cameraNum, Number(value)).catch((e) => alert(e.message));
}
</script>
