import { defineStore } from 'pinia'
import { useAuthStore } from './auth'
import { getMyPermissions } from '@/api/permissions'

// Кэш TTL: 30s. Через 30s router-guard и v-permission-scope перезагрузят
// permissions. Достаточно, чтобы изменения админа дошли до юзера, и не
// слишком часто — endpoint /permissions/my делает 1 SQL.
const CACHE_TTL_MS = 30 * 1000

/**
 * Разбирает ответ /permissions/my, поддерживая два формата:
 *   - новый объект: { mode, permissions, denied, banned, banReason }
 *   - старый массив: [{ key, value }]
 *
 * @param {unknown} data
 * @returns {{ mode: string, effective: Object, denied: Set, banned: boolean, banReason: string|null }}
 */
function parsePermissionsResponse(data) {
  // Новый формат
  if (data && typeof data === 'object' && !Array.isArray(data) && 'mode' in data) {
    const mode = data.mode || 'normal'
    const effective = {}
    const rawPerms = Array.isArray(data.permissions) ? data.permissions : []
    rawPerms.forEach(p => {
      if (p && p.key) {
        effective[p.key] = { value: p.value || 'deny', source: p.source || 'base' }
      }
    })
    const denied = new Set(Array.isArray(data.denied) ? data.denied : [])
    const banned = Boolean(data.banned)
    const banReason = data.banReason ?? data.ban_reason ?? null
    return { mode, effective, denied, banned, banReason }
  }

  // Старый формат — массив [{key, value}]
  const effective = {}
  const rawPerms = Array.isArray(data) ? data : []
  rawPerms.forEach(p => {
    if (p && p.key) {
      effective[p.key] = { value: p.value || 'deny', source: 'base' }
    }
  })
  // Режим определяем через auth store (isSuperAdmin)
  const auth = useAuthStore()
  const mode = auth.isSuperAdmin ? 'super' : 'normal'
  return { mode, effective, denied: new Set(), banned: false, banReason: null }
}

export const usePermissionsStore = defineStore('permissions', {
  state: () => ({
    /**
     * Эффективная карта: { [key]: { value: 'allow'|'deny', source: string } }
     */
    effective: {},
    /**
     * Режим: 'super' | 'admin' | 'normal' | 'banned'
     */
    mode: 'normal',
    /**
     * Ключи, недоступные для admin-режима (бэк сигнализирует через denied).
     */
    denied: new Set(),
    banned: false,
    banReason: null,
    loaded: false,
    fetchedAt: 0,
    inflight: null,
    /**
     * Плоская карта { key: value } — сохраняем для обратной совместимости
     * с кодом, который напрямую читает store.permissions[key].
     * Синхронизируется при каждом fetchPermissions.
     */
    permissions: {},
  }),

  getters: {
    /**
     * Проверяет, разрешён ли ключ для текущего пользователя.
     *
     * Семантика по mode:
     *   super  -> всегда true
     *   admin  -> true если ключ не в denied
     *   normal -> true если effective[key].value === 'allow'
     *   banned -> всегда false
     *
     * @param {string} key
     * @returns {boolean}
     */
    hasPermission: (state) => (key) => {
      switch (state.mode) {
        case 'super':
          return true
        case 'admin':
          return !state.denied.has(key)
        case 'banned':
          return false
        default: // 'normal'
          return state.effective[key]?.value === 'allow'
      }
    },

    /**
     * Возвращает источник права: 'role'|'group'|'override'|'admin'|'base'|null.
     * Используется в админ-UI для индикатора источника.
     *
     * @param {string} key
     * @returns {string|null}
     */
    permissionSource: (state) => (key) => {
      return state.effective[key]?.source ?? null
    },

    isStale: (state) => {
      if (!state.loaded) return true
      return Date.now() - state.fetchedAt > CACHE_TTL_MS
    },
  },

  actions: {
    /**
     * Загружает permissions с бэка. Если уже идёт fetch — возвращает
     * текущий promise (защита от дублирования при параллельных вызовах).
     * @param {boolean} force — игнорировать кэш и форсить запрос.
     */
    async fetchPermissions(force = false) {
      if (this.inflight) return this.inflight
      if (!force && !this.isStale) return Promise.resolve()
      this.inflight = (async () => {
        try {
          const data = await getMyPermissions()
          const parsed = parsePermissionsResponse(data)
          this.mode = parsed.mode
          this.effective = parsed.effective
          this.denied = parsed.denied
          this.banned = parsed.banned
          this.banReason = parsed.banReason
          // Синхронизируем плоскую карту для обратной совместимости
          this.permissions = {}
          Object.entries(parsed.effective).forEach(([k, v]) => {
            this.permissions[k] = v.value
          })
          this.loaded = true
          this.fetchedAt = Date.now()
        } catch {
          // игнорируем ошибку — permissions остаются как есть
        } finally {
          this.inflight = null
        }
      })()
      return this.inflight
    },

    invalidate() {
      this.fetchedAt = 0
    },

    clearPermissions() {
      this.effective = {}
      this.permissions = {}
      this.mode = 'normal'
      this.denied = new Set()
      this.banned = false
      this.banReason = null
      this.loaded = false
      this.fetchedAt = 0
    },
  },
})
