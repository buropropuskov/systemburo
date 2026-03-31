import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useUiStore = defineStore('ui', () => {
  const toasts = ref([])
  const sidebarExpanded = ref(false)

  function showToast(message, type = 'info', duration = 3000) {
    const id = Date.now()
    toasts.value.push({ id, message, type })
    setTimeout(() => {
      toasts.value = toasts.value.filter(t => t.id !== id)
    }, duration)
  }

  function success(message) { showToast(message, 'success') }
  function error(message) { showToast(message, 'error', 5000) }
  function warning(message) { showToast(message, 'warning') }

  return { toasts, sidebarExpanded, showToast, success, error, warning }
})
