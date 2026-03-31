import { useUiStore } from '@/stores/ui'

export function useToast() {
  const ui = useUiStore()
  return {
    success: ui.success,
    error: ui.error,
    warning: ui.warning,
    show: ui.showToast,
  }
}
