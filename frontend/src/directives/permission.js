import { usePermissionsStore } from '@/stores/permissions'

export const vPermission = {
  mounted(el, binding) {
    const store = usePermissionsStore()
    if (!store.hasPermission(binding.value)) {
      el.style.display = 'none'
    }
  },
  updated(el, binding) {
    const store = usePermissionsStore()
    el.style.display = store.hasPermission(binding.value) ? '' : 'none'
  },
}
