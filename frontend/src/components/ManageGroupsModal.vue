<template>
  <Modal title="Manage Groups" wide @close="$emit('close')">
    <div v-if="camera.groups.length === 0" class="no-groups-row">
      <span class="no-groups-msg">no groups</span>
      <button @click="adding = true">Add Group</button>
    </div>

    <div v-else class="groups-grid-wrap">
      <div class="groups-grid" :style="gridStyle">
        <!-- group name row -->
        <div class="g-cell corner" />
        <div class="g-cell corner" />
        <div v-for="g in camera.groups" :key="'name' + g.id" class="g-cell">
          <input type="text" :value="g.name" @change="renameGroup(g, $event.target.value)" />
        </div>
        <div class="g-cell corner add-cell">
          <button @click="adding = true">Add Group</button>
        </div>

        <!-- color + delete row -->
        <div class="g-cell corner" />
        <div class="g-cell corner" />
        <div v-for="g in camera.groups" :key="'ctl' + g.id" class="g-cell g-controls">
          <select
            :value="g.color"
            :style="{ borderColor: GROUP_COLOR_HEX[g.color] }"
            @change="setColor(g, $event.target.value)"
          >
            <option v-for="c in colorOptions" :key="c" :value="c">{{ c }}</option>
          </select>
          <button class="icon-btn" title="Delete group" @click="deleteGroup(g)">🗑</button>
        </div>
        <div class="g-cell corner" />

        <!-- preset rows -->
        <template v-for="p in camera.presets" :key="'row' + p.id">
          <div class="g-cell">
            <img :src="`${p.thumbnailUrl}?v=${p.thumbnailVersion}`" :alt="p.name" />
          </div>
          <div class="g-cell">
            <input type="text" :value="p.name" @change="renamePreset(p, $event.target.value)" />
          </div>
          <div v-for="g in camera.groups" :key="'m' + g.id + '_' + p.id" class="g-cell g-check-cell">
            <input
              type="checkbox"
              class="g-checkbox"
              :style="{ '--group-color': GROUP_COLOR_HEX[g.color] }"
              :checked="isMember(g, p)"
              @change="toggleMember(g, p, $event.target.checked)"
            />
          </div>
          <div class="g-cell corner" />
        </template>
      </div>
    </div>

    <ConfirmDialog
      v-if="deletingGroup"
      title="Delete Group"
      :message="`Delete group &quot;${deletingGroup.name}&quot;?`"
      @confirm="confirmDeleteGroup"
      @cancel="deletingGroup = null"
    />

    <PromptModal
      v-if="adding"
      title="Add Group"
      label="Group name"
      initial-value=""
      submit-label="Add"
      :on-submit="doAddGroup"
      @close="adding = false"
    />
  </Modal>
</template>

<script setup>
import { computed, ref } from 'vue';
import Modal from './Modal.vue';
import PromptModal from './PromptModal.vue';
import ConfirmDialog from './ConfirmDialog.vue';
import { api } from '../api.js';
import { GROUP_COLOR_HEX } from '../colors.js';

const props = defineProps({ camera: Object });
defineEmits(['close']);

const colorOptions = Object.keys(GROUP_COLOR_HEX);

const gridStyle = computed(() => ({
  gridTemplateColumns: `110px 150px repeat(${props.camera.groups.length}, minmax(130px, auto)) 120px`,
}));

function isMember(g, p) {
  return g.members.some((m) => m.presetId === p.id);
}

function renameGroup(g, name) {
  if (!name.trim()) return;
  api.updateGroup(props.camera.id, g.id, { name: name.trim() }).catch((e) => alert(e.message));
}
function setColor(g, color) {
  api.updateGroup(props.camera.id, g.id, { color }).catch((e) => alert(e.message));
}
function toggleMember(g, p, checked) {
  api.setMember(props.camera.id, g.id, p.id, { inGroup: checked }).catch((e) => alert(e.message));
}
function renamePreset(p, name) {
  if (!name.trim()) return;
  api.renamePreset(props.camera.id, p.id, name.trim()).catch((e) => alert(e.message));
}

const deletingGroup = ref(null);
function deleteGroup(g) {
  deletingGroup.value = g;
}
async function confirmDeleteGroup() {
  await api.deleteGroup(props.camera.id, deletingGroup.value.id);
  deletingGroup.value = null;
}

const adding = ref(false);
async function doAddGroup(name) {
  await api.addGroup(props.camera.id, name);
}
</script>
