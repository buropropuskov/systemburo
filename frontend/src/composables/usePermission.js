import { usePermissionsStore } from '@/stores/permissions'

/**
 * Composable для проверки прав в Composition API.
 *
 * Пример:
 *   const { can, sourceOf } = usePermission()
 *   const canExport = computed(() => can('action.export.applications'))
 *   // Динамический ключ (например, для таблиц):
 *   const canViewTable = computed(() => can(`table.${slug}.view`))
 *
 * @returns {{ can: (key: string) => boolean, sourceOf: (key: string) => string|null }}
 */
export function usePermission() {
  const store = usePermissionsStore()

  /**
   * Реактивная проверка права. Работает с любыми ключами, в том числе
   * с динамическими вида `table.<slug>.<verb>`.
   *
   * @param {string} key
   * @returns {boolean}
   */
  function can(key) {
    return store.hasPermission(key)
  }

  /**
   * Возвращает источник права: 'role'|'group'|'override'|'admin'|'base'|null.
   *
   * @param {string} key
   * @returns {string|null}
   */
  function sourceOf(key) {
    return store.permissionSource(key)
  }

  return { can, sourceOf }
}
