<template>
  <Modal :title="`Manage ${camera.name}`" tall @close="$emit('close')">
    <div class="manage-shell">
      <div class="manage-sidebar">
        <button class="manage-tab" :class="{ active: tab === 'general' }" @click="tab = 'general'">General</button>
        <button class="manage-tab" :class="{ active: tab === 'names' }" @click="tab = 'names'">Preset Names</button>
        <button class="manage-tab" :class="{ active: tab === 'groups' }" @click="tab = 'groups'">Groups</button>
        <button class="manage-tab" :class="{ active: tab === 'layout' }" @click="tab = 'layout'">Layout &amp; Order</button>
      </div>

      <div class="manage-content">
        <template v-if="tab === 'general'">
          <div class="field">
            <label>Camera name</label>
            <input type="text" v-model="nameDraft" @blur="saveName" @keyup.enter="$event.target.blur()" />
          </div>
          <div class="manage-info-row"><span>Host</span><span class="metric-block">{{ camera.ip }}:{{ camera.port }}</span></div>
          <div class="manage-info-row"><span>Tally source</span><span class="metric-block">{{ camera.tallySource }}</span></div>
          <div class="manage-info-row"><span>Status</span><span class="metric-block">{{ camera.status }}</span></div>
          <div class="manage-info-row"><span>Presets</span><span class="metric-block">{{ camera.presets.length }}</span></div>
          <div class="manage-info-row"><span>Groups</span><span class="metric-block">{{ camera.groups.length }}</span></div>
          <div class="manage-info-row"><span>Columns</span><span class="metric-block">{{ camera.columnCount }}</span></div>
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
            <label>Columns</label>
            <select :value="camera.columnCount" @change="setColumnCount($event.target.value)">
              <option v-for="n in 6" :key="n" :value="n">{{ n }}</option>
            </select>
          </div>
          <p class="metric-block">Drag-to-reorder is coming soon.</p>
          <div class="manage-preset-grid" :style="gridStyle">
            <ManagePresetTile v-for="p in camera.presets" :key="p.id" :preset="p" />
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
import { computed, ref, watch } from 'vue';
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
function toggleMember(g, p, checked) {
  api.setMember(props.camera.id, g.id, p.id, { inGroup: checked }).catch((e) => alert(e.message));
}

function setColumnCount(value) {
  api.updateCamera(props.camera.id, { columnCount: Number(value) }).catch((e) => alert(e.message));
}

const deleting = ref(false);
async function doDelete() {
  await api.deleteCamera(props.camera.id);
  deleting.value = false;
  emit('close');
}
</script>
