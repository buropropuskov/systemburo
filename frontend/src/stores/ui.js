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
  function info(message) { showToast(message, 'info') }

  // Глобальная модалка подтверждения. Возвращает Promise<boolean>.
  // Использование: const ok = await ui.confirm({ message: '...' })
  const confirmState = ref(null)

  function confirm({
    title = 'Подтверждение',
    message,
    confirmText = 'Удалить',
    cancelText = 'Отмена',
    danger = true,
  } = {}) {
    return new Promise((resolve) => {
      confirmState.value = {
        title,
        message,
        confirmText,
        cancelText,
        danger,
        resolve,
      }
    })
  }

  function resolveConfirm(value) {
    if (confirmState.value) {
      confirmState.value.resolve(value)
      confirmState.value = null
    }
  }

  return {
    toasts,
    sidebarExpanded,
    showToast,
    success,
    error,
    warning,
    info,
    confirmState,
    confirm,
    resolveConfirm,
  }
})
