import { createApp } from 'vue';
import App from './App.vue';
import './style.css';

createApp(App).mount('#app');

// Flag when a queue-modifier is held (see PresetThumbnail onClick)
const syncMod = (e) =>
  document.body.classList.toggle('mod-held', e.ctrlKey || e.shiftKey || e.altKey || e.metaKey);
['keydown', 'keyup', 'mousemove', 'mousedown'].forEach((t) => window.addEventListener(t, syncMod));
window.addEventListener('blur', () => document.body.classList.remove('mod-held'));

// Suppress hover effects while a drag is in progress
window.addEventListener('dragstart', () => document.body.classList.add('is-dragging'));
window.addEventListener('dragend', () => document.body.classList.remove('is-dragging'));
