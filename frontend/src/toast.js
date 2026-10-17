import { reactive } from 'vue';

export const toast = reactive({ message: '', id: 0 });
let timer;

export function showToast(message, ms = 3000) {
  clearTimeout(timer);
  toast.message = message;
  toast.id++;
  timer = setTimeout(() => (toast.message = ''), ms);
}
