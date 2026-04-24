import { defineStore } from 'pinia'

const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL || '') + '/api'

/**
 * Глобальный стор статуса технических работ.
 * Заполняется в bootstrap (main.js) через fetchStatus(), потом
 * перечитывается:
 *   - на странице /maintenance каждые 30 секунд (auto-refresh)
 *   - после PUT /admin/maintenance (моментально)
 */
export const useMaintenanceStore = defineStore('maintenance', {
  state: () => ({
    enabled: false,
    message: '',
    startedAt: '',
    supportEmail: '',
    loaded: false,
  }),
  actions: {
    /**
     * Публичный GET без auth - доступен и неавторизованному юзеру на /login.
     */
    async fetchStatus() {
      try {
        const r = await fetch(`${API_BASE_URL}/settings/maintenance`, {
          method: 'GET',
          credentials: 'include',
          headers: { 'Accept': 'application/json' },
        })
        if (!r.ok) {
          this.loaded = true
          return
        }
        const body = await r.json()
        const data = body && typeof body === 'object' && 'success' in body ? body.data : body
        this.enabled = !!data?.enabled
        this.message = data?.message || ''
        this.startedAt = data?.started_at || ''
        this.supportEmail = data?.support_email || ''
      } catch {
        // Сетевая ошибка - считаем, что режим не включён, иначе положим сайт
        // неверным результатом.
      } finally {
        this.loaded = true
      }
    },
    setFromPayload(payload) {
      this.enabled = !!payload?.enabled
      this.message = payload?.message || ''
      this.startedAt = payload?.started_at || ''
      this.supportEmail = payload?.support_email || ''
      this.loaded = true
    },
  },
})
