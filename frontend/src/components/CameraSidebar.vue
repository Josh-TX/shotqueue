<template>
  <div class="camera-sidebar">
    <label
      v-for="camera in sortedCameras"
      :key="camera.cameraNum"
      class="camera-sidebar-item"
      :title="`Show ${cameraName(camera)}`"
    >
      <input type="checkbox" :checked="!camera.isHidden" @change="setVisible(camera, $event.target.checked)" />
      <span class="camera-sidebar-label">Cam {{ camera.cameraNum }}</span>
    </label>
  </div>
</template>

<script setup>
import { api } from '../api.js';
import { cameraName, sortedCameras } from '../store.js';

function setVisible(camera, visible) {
  api.setCameraHidden(camera.cameraNum, !visible).catch((e) => alert(e.message));
}
</script>
