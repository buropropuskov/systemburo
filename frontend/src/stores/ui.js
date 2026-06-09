import { defineStore } from 'pinia'
import { ref, watch } from 'vue'

const NAV_PREFS_KEY = 'nav-prefs'

function loadNavPrefs() {
  try {
    return JSON.parse(localStorage.getItem(NAV_PREFS_KEY)) || {}
  } catch {
    return {}
  }
}

export const useUiStore = defineStore('ui', () => {
  const toasts = ref([])

  // Состояния рельса навигации (#510), персист в localStorage:
  // sidebarExpanded = пин (рельс закреплён раскрытым), sidebarHidden = full-hide.
  const navPrefs = loadNavPrefs()
  const sidebarExpanded = ref(navPrefs.pinned ?? false)
  const sidebarHidden = ref(navPrefs.hidden ?? false)

  watch([sidebarExpanded, sidebarHidden], ([pinned, hidden]) => {
    try {
      localStorage.setItem(NAV_PREFS_KEY, JSON.stringify({ pinned, hidden }))
    } catch {
      // localStorage недоступен (приватный режим) - персист best-effort, не критично.
    }
  })

  function toggleSidebarPinned() {
    sidebarExpanded.value = !sidebarExpanded.value
  }

  function hideSidebar() {
    sidebarHidden.value = true
  }

  function showSidebar() {
    sidebarHidden.value = false
  }

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
    sidebarHidden,
    toggleSidebarPinned,
    hideSidebar,
    showSidebar,
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
