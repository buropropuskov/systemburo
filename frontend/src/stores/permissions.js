import { defineStore } from 'pinia'
import { useAuthStore } from './auth'
import { getMyPermissions } from '@/api/permissions'

// Кэш TTL: 30s. Через 30s router-guard и v-permission-scope перезагрузят
// permissions. Достаточно, чтобы изменения админа дошли до юзера, и не
// слишком часто — endpoint /permissions/my делает 1 SQL.
const CACHE_TTL_MS = 30 * 1000

export const usePermissionsStore = defineStore('permissions', {
  state: () => ({
    permissions: {},
    loaded: false,
    fetchedAt: 0,
    inflight: null,
  }),

  getters: {
    hasPermission: (state) => (key) => {
      const auth = useAuthStore()
      if (auth.isSuperAdmin) return true
      return state.permissions[key] === 'allow'
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
          this.permissions = {}
          if (Array.isArray(data)) {
            data.forEach(p => { this.permissions[p.key] = p.value })
          }
          this.loaded = true
          this.fetchedAt = Date.now()
        } catch {
          // ignore — permissions will stay empty
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
      this.permissions = {}
      this.loaded = false
      this.fetchedAt = 0
    },
  },
})
