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
  // Состояния рельса навигации (#510), персист в localStorage:
  // sidebarExpanded = пин (рельс закреплён раскрытым), sidebarHidden = full-hide.
  const navPrefs = loadNavPrefs()
  const sidebarExpanded = ref(navPrefs.pinned ?? false)
  const sidebarHidden = ref(navPrefs.hidden ?? false)
  // Временный оверлейный разворот рельса (онбординг-тур). Не персистится и не
  // влияет на --nav-ml: рельс расширяется поверх контента, без reflow.
  const tourForceExpand = ref(false)
  // Идёт онбординг-тур. Живёт в ui, а не в сторе онбординга, чтобы читатели
  // (плашки уведомлений) не тянули за собой весь модуль тура с его роутером.
  const tourActive = ref(false)

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
    tourActive,
    sidebarExpanded,
    sidebarHidden,
    tourForceExpand,
    toggleSidebarPinned,
    hideSidebar,
    showSidebar,
    confirmState,
    confirm,
    resolveConfirm,
  }
})
