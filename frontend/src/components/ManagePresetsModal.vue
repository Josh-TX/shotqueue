<template>
  <Modal :title="`${camera.name}`" tall :min-width="modalWidth" @close="$emit('close')">
    <div class="manage-shell">
      <div class="manage-sidebar">
        <button class="manage-tab" :class="{ active: tab === 'general' }" @click="tab = 'general'">General</button>
        <button class="manage-tab" :class="{ active: tab === 'names' }" @click="tab = 'names'">Preset Names</button>
        <button class="manage-tab" :class="{ active: tab === 'groups' }" @click="tab = 'groups'">Groups</button>
        <button class="manage-tab" :class="{ active: tab === 'layout' }" @click="tab = 'layout'">Layout &amp; Order</button>
      </div>

      <div class="manage-content" ref="contentEl" @dragover.prevent="onContentDragOver">
        <template v-if="tab === 'general'">
          <div class="field">
            <label>Camera name</label>
            <input type="text" v-model="nameDraft" @blur="saveName" @keyup.enter="$event.target.blur()" />
          </div>
          <div class="manage-info-row">
            <span>Host</span>
            <input type="text" v-model="hostDraft" @blur="saveHost" @keyup.enter="$event.target.blur()" />
          </div>
          <div class="manage-info-row">
            <span>Port</span>
            <input type="text" v-model="portDraft" @blur="savePort" @keyup.enter="$event.target.blur()" />
          </div>
          <div class="manage-info-row">
            <span>Tally source</span>
            <input type="number" v-model.number="tallyDraft" @blur="saveTally" @keyup.enter="$event.target.blur()" />
          </div>
          <div class="manage-info-row">
            <span>Thumbnail Columns</span>
            <select :value="camera.columnCount" @change="setColumnCount($event.target.value)">
              <option v-for="n in 6" :key="n" :value="n">{{ n }}</option>
            </select>
          </div>
          <div class="manage-info-row"><span>Presets</span><span class="metric-block">{{ camera.presets.length }}</span></div>
          <div class="manage-info-row"><span>Groups</span><span class="metric-block">{{ camera.groups.length }}</span></div>
          <div class="modal-actions">
            <button class="danger" @click="deleting = true">Delete Camera</button>
          </div>
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
                <option value="random">Random</option>
                <option value="sequence">Sequence</option>
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

    <ConfirmDialog
      v-if="deleting"
      title="Delete Camera"
      :message="`Delete &quot;${camera.name}&quot; and all its presets and groups?`"
      @confirm="doDelete"
      @cancel="deleting = false"
    />
  </Modal>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import Modal from './Modal.vue';
import ManagePresetTile from './ManagePresetTile.vue';
import ConfirmDialog from './ConfirmDialog.vue';
import { api } from '../api.js';
import { groupColor } from '../colors.js';

const props = defineProps({ camera: Object, unitWidth: Number });
const emit = defineEmits(['close']);

const tab = ref('general');

const gridStyle = computed(() => ({
  gridTemplateColumns: `repeat(${props.camera.columnCount}, ${props.unitWidth}px)`,
}));

const manageShellOverhead = 230; // sidebar + gaps + modal padding + scrollbar
const modalWidth = computed(() => {
  const gridWidth = props.camera.columnCount * props.unitWidth + (props.camera.columnCount - 1) * 10;
  return Math.max(620, gridWidth + manageShellOverhead);
});

const nameDraft = ref(props.camera.name);
watch(() => props.camera.name, (n) => (nameDraft.value = n));
function saveName() {
  const name = nameDraft.value.trim();
  if (!name) {
    nameDraft.value = props.camera.name;
    return;
  }
  api.updateCamera(props.camera.id, { name }).catch((e) => alert(e.message));
}

const hostDraft = ref(props.camera.ip);
watch(() => props.camera.ip, (h) => (hostDraft.value = h));
function saveHost() {
  const host = hostDraft.value.trim();
  if (!host) {
    hostDraft.value = props.camera.ip;
    return;
  }
  api.updateCamera(props.camera.id, { host }).catch((e) => alert(e.message));
}

const portDraft = ref(props.camera.port);
watch(() => props.camera.port, (p) => (portDraft.value = p));
function savePort() {
  const port = String(portDraft.value).trim();
  if (!port) {
    portDraft.value = props.camera.port;
    return;
  }
  api.updateCamera(props.camera.id, { port }).catch((e) => alert(e.message));
}

const tallyDraft = ref(props.camera.tallySource);
watch(() => props.camera.tallySource, (t) => (tallyDraft.value = t));
function saveTally() {
  api.updateCamera(props.camera.id, { tallySource: tallyDraft.value }).catch((e) => alert(e.message));
}

function renamePreset(p, name) {
  api.renamePreset(props.camera.id, p.id, name).catch((e) => alert(e.message));
}

function setGroupCount(value) {
  api.setGroupCount(props.camera.id, Number(value)).catch((e) => alert(e.message));
}
function renameGroup(g, name) {
  if (!name.trim()) return;
  api.updateGroup(props.camera.id, g.id, { name: name.trim() }).catch((e) => alert(e.message));
}
function setGroupMode(g, value) {
  api.updateGroup(props.camera.id, g.id, { isSequence: value === 'sequence' }).catch((e) => alert(e.message));
}
function toggleMember(g, p, checked) {
  api.setMember(props.camera.id, g.id, p.id, { inGroup: checked }).catch((e) => alert(e.message));
}

function setColumnCount(value) {
  api.updateCamera(props.camera.id, { columnCount: Number(value) }).catch((e) => alert(e.message));
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
  api.reorderPresets(props.camera.id, props.camera.presets.map((p) => p.id)).catch((e) => alert(e.message));
}

onBeforeUnmount(stopAutoScroll);

const deleting = ref(false);
async function doDelete() {
  await api.deleteCamera(props.camera.id);
  deleting.value = false;
  emit('close');
}
</script>
