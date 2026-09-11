<template>
  <Modal :title="cameraLabel" tall :min-width="modalWidth" @close="$emit('close')">
    <div class="manage-shell">
      <div class="manage-sidebar">
        <button class="manage-tab" :class="{ active: tab === 'general' }" @click="tab = 'general'">General</button>
        <button class="manage-tab" :class="{ active: tab === 'names' }" @click="tab = 'names'">Preset Names</button>
        <button class="manage-tab" :class="{ active: tab === 'groups' }" @click="tab = 'groups'">Groups</button>
        <button class="manage-tab" :class="{ active: tab === 'layout' }" @click="tab = 'layout'">Layout &amp; Order</button>
      </div>

      <div class="manage-content" ref="contentEl" @dragover.prevent="onContentDragOver">
        <template v-if="tab === 'general'">
          <p class="help-text">Some preset operations (reposition/delete) are only available by right clicking the preset.</p>
          <p class="help-text">Connection info and hiding/deleting this camera are managed on the Settings page.</p>
          <div class="manage-info-row">
            <span>Thumbnail Columns</span>
            <select :value="camera.columnCount" @change="setColumnCount($event.target.value)">
              <option v-for="n in 6" :key="n" :value="n">{{ n }}</option>
            </select>
          </div>
          <div class="manage-info-row"><span>Presets</span><span class="metric-block">{{ camera.presets.length }}</span></div>
          <div class="manage-info-row"><span>Groups</span><span class="metric-block">{{ camera.groups.length }}</span></div>
        </template>

        <template v-else-if="tab === 'names'">
          <div class="manage-preset-grid" :style="gridStyle">
            <ManagePresetTile v-for="p in camera.presets" :key="p.id" :preset="p">
              <template #title>
                <input
                  type="text"
                  class="preset-thumb-name-input"
                  :value="p.name"
                  @blur="renamePreset(p, $event.target.value)"
                  @keyup.enter="$event.target.blur()"
                />
              </template>
            </ManagePresetTile>
            <div v-if="camera.presets.length === 0" class="no-presets">No presets yet</div>
          </div>
        </template>

        <template v-else-if="tab === 'groups'">
          <div class="manage-group-count-row">
            <label>Group count</label>
            <select :value="camera.groups.length" @change="setGroupCount($event.target.value)">
              <option v-for="n in 4" :key="n" :value="n">{{ n }}</option>
            </select>
          </div>

          <div class="manage-group-names">
            <div v-for="(g, i) in camera.groups" :key="g.id" class="manage-group-name-row">
              <span class="manage-group-swatch" :style="{ background: groupColor(i) }" />
              <input
                type="text"
                :value="g.name"
                @blur="renameGroup(g, $event.target.value)"
                @keyup.enter="$event.target.blur()"
              />
              <select :value="g.isSequence ? 'sequence' : 'random'" @change="setGroupMode(g, $event.target.value)">
                <option value="random">Random Mode</option>
                <option value="sequence">Sequence Mode</option>
              </select>
            </div>
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
                    :style="{ '--group-color': groupColor(i) }"
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

        <template v-else-if="tab === 'layout'">
          <div class="manage-group-count-row">
            <label>Thumbnail Columns</label>
            <select :value="camera.columnCount" @change="setColumnCount($event.target.value)">
              <option v-for="n in 6" :key="n" :value="n">{{ n }}</option>
            </select>
          </div>
          <p class="metric-block">Drag a tile to reorder.</p>
          <div class="manage-preset-grid" :style="gridStyle">
            <ManagePresetTile
              v-for="(p, i) in camera.presets"
              :key="p.id"
              :preset="p"
              draggable="true"
              class="draggable-tile"
              :class="{ dragging: dragIndex === i }"
              @dragstart="onDragStart(i, $event)"
              @dragenter="onDragEnter(i)"
              @dragover.prevent
              @dragend="onDragEnd"
            />
            <div v-if="camera.presets.length === 0" class="no-presets">No presets yet</div>
          </div>
        </template>
      </div>
    </div>
  </Modal>
</template>

<script setup>
import { computed, onBeforeUnmount, ref } from 'vue';
import Modal from './Modal.vue';
import ManagePresetTile from './ManagePresetTile.vue';
import { api } from '../api.js';
import { cameraName } from '../store.js';
import { groupColor } from '../colors.js';

const props = defineProps({ camera: Object, unitWidth: Number });
defineEmits(['close']);

const tab = ref('general');

const gridStyle = computed(() => ({
  gridTemplateColumns: `repeat(${props.camera.columnCount}, ${props.unitWidth}px)`,
}));

const manageShellOverhead = 230; // sidebar + gaps + modal padding + scrollbar
const modalWidth = computed(() => {
  const gridWidth = props.camera.columnCount * props.unitWidth + (props.camera.columnCount - 1) * 10;
  return Math.max(620, gridWidth + manageShellOverhead);
});

const cameraLabel = computed(() => cameraName(props.camera));

function renamePreset(p, name) {
  api.renamePreset(props.camera.cameraNum, p.id, name).catch((e) => alert(e.message));
}

function setGroupCount(value) {
  api.setGroupCount(props.camera.cameraNum, Number(value)).catch((e) => alert(e.message));
}
function renameGroup(g, name) {
  if (!name.trim()) return;
  api.updateGroup(props.camera.cameraNum, g.id, { name: name.trim() }).catch((e) => alert(e.message));
}
function setGroupMode(g, value) {
  api.updateGroup(props.camera.cameraNum, g.id, { isSequence: value === 'sequence' }).catch((e) => alert(e.message));
}
function toggleMember(g, p, checked) {
  api.setMember(props.camera.cameraNum, g.id, p.id, { inGroup: checked }).catch((e) => alert(e.message));
}

function setColumnCount(value) {
  api.setColumnCount(props.camera.cameraNum, Number(value)).catch((e) => alert(e.message));
}

const dragIndex = ref(null);
const contentEl = ref(null);
const autoScrollMargin = 50;
const autoScrollMaxSpeed = 14;
let autoScrollSpeed = 0;
let autoScrollRAF = null;

function autoScrollStep() {
  if (contentEl.value) contentEl.value.scrollTop += autoScrollSpeed;
  autoScrollRAF = requestAnimationFrame(autoScrollStep);
}
function stopAutoScroll() {
  autoScrollSpeed = 0;
  if (autoScrollRAF !== null) cancelAnimationFrame(autoScrollRAF);
  autoScrollRAF = null;
}
function onContentDragOver(event) {
  if (dragIndex.value === null || !contentEl.value) return;
  const rect = contentEl.value.getBoundingClientRect();
  let speed = 0;
  if (event.clientY < rect.top + autoScrollMargin) {
    speed = -Math.ceil(((rect.top + autoScrollMargin - event.clientY) / autoScrollMargin) * autoScrollMaxSpeed);
  } else if (event.clientY > rect.bottom - autoScrollMargin) {
    speed = Math.ceil(((event.clientY - (rect.bottom - autoScrollMargin)) / autoScrollMargin) * autoScrollMaxSpeed);
  }
  autoScrollSpeed = speed;
  if (speed !== 0 && autoScrollRAF === null) autoScrollRAF = requestAnimationFrame(autoScrollStep);
  else if (speed === 0) stopAutoScroll();
}

function onDragStart(i, event) {
  dragIndex.value = i;
  event.dataTransfer.effectAllowed = 'move';
}
function onDragEnter(i) {
  if (dragIndex.value === null || dragIndex.value === i) return;
  const presets = props.camera.presets;
  const [moved] = presets.splice(dragIndex.value, 1);
  presets.splice(i, 0, moved);
  dragIndex.value = i;
}
function onDragEnd() {
  dragIndex.value = null;
  stopAutoScroll();
  api.reorderPresets(props.camera.cameraNum, props.camera.presets.map((p) => p.id)).catch((e) => alert(e.message));
}

onBeforeUnmount(stopAutoScroll);
</script>
