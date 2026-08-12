import { defineStore } from 'pinia'
import { apiRequest } from '@/api/client'

/**
 * Глобальный стор справочника компаний.
 *
 * Аналог organizations store. Поддерживает два представления:
 *   - items: лёгкий список { id, name } для dropdown'ов
 *   - itemsWithUsers: расширенный с user_count для CompaniesManagement
 *
 * После любой CUD-операции через action'ы автоматически синхронизируются
 * оба списка - все компоненты ЛК получают свежие данные без ручных fetch'ей.
 */
export const useCompaniesStore = defineStore('companies', {
  state: () => ({
    items: [],
    itemsWithUsers: [],
    isLoading: false,
    error: null,
  }),

  getters: {
    findById: (state) => (id) => state.items.find(c => c.id === id) || null,
    nameById: (state) => (id) => {
      const comp = state.items.find(c => c.id === id)
      return comp ? comp.name : ''
    },
  },

  actions: {
    /**
     * Загружает базовый список { id, name } (используется UserControl/dropdown).
     */
    async fetchCompanies() {
      try {
        const response = await apiRequest('/companies')
        if (response.ok) {
          this.items = await response.json()
        }
      } catch (err) {
        console.error('Error fetching companies:', err)
        this.error = err
      }
    },

    /**
     * Зеркало organizations: список закрыт page.admin.directories (#2002), но зовётся
     * и из UserControl, где 403 у админа пользователей штатный - молчим, см. подробный
     * разбор в stores/organizations.js.
     * @param {boolean} includeArchived - включить архивные компании (is_active=false)
     */
    async fetchCompaniesWithUsers(includeArchived = false) {
      try {
        const qs = includeArchived ? '?include_archived=true' : ''
        const response = await apiRequest(`/companies/with-users-extended${qs}`, { silent403: true })
        if (response.ok) {
          const data = await response.json()
          this.itemsWithUsers = data.map(comp => ({
            ...comp,
            originalName: comp.name,
          }))
        }
      } catch (err) {
        console.error('Error fetching companies with users:', err)
        this.error = err
      }
    },

    /**
     * @param {boolean} includeArchived
     */
    async refresh(includeArchived = false) {
      this.isLoading = true
      try {
        await Promise.all([
          this.fetchCompanies(),
          this.fetchCompaniesWithUsers(includeArchived),
        ])
      } finally {
        this.isLoading = false
      }
    },

    /**
     * @param {{ name: string }} payload
     * @param {{ includeArchived?: boolean }} [opts]
     * @returns {Promise<{ ok: boolean, data?: object, message?: string }>}
     */
    async createCompany(payload, { includeArchived = false } = {}) {
      try {
        const response = await apiRequest('/companies', {
          method: 'POST',
          body: JSON.stringify(payload),
        })
        if (response.ok) {
          const data = await response.json()
          await this.refresh(includeArchived)
          return { ok: true, data }
        }
        const error = await response.json().catch(() => ({}))
        return { ok: false, message: error.message }
      } catch (err) {
        console.error('Error creating company:', err)
        return { ok: false, message: 'Ошибка сети' }
      }
    },

    /**
     * @param {number} id
     * @param {{ name: string }} payload
     * @param {{ includeArchived?: boolean }} [opts]
     */
    async updateCompany(id, payload, { includeArchived = false } = {}) {
      try {
        const response = await apiRequest(`/companies/${id}`, {
          method: 'PUT',
          body: JSON.stringify(payload),
        })
        if (response.ok) {
          await this.refresh(includeArchived)
          return { ok: true }
        }
        const error = await response.json().catch(() => ({}))
        return { ok: false, message: error.message }
      } catch (err) {
        console.error('Error updating company:', err)
        return { ok: false, message: 'Ошибка сети' }
      }
    },

    /**
     * Архивирует компанию (soft-delete, is_active=false).
     * @param {number} id
     * @param {{ includeArchived?: boolean }} [opts]
     */
    async deleteCompany(id, { includeArchived = false } = {}) {
      try {
        const response = await apiRequest(`/companies/${id}`, {
          method: 'DELETE',
        })
        if (response.ok) {
          await this.refresh(includeArchived)
          return { ok: true }
        }
        const error = await response.json().catch(() => ({}))
        return { ok: false, message: error.message }
      } catch (err) {
        console.error('Error deleting company:', err)
        return { ok: false, message: 'Ошибка сети' }
      }
    },

    /**
     * Восстанавливает компанию из архива (is_active=true).
     * @param {number} id
     * @param {{ includeArchived?: boolean }} [opts]
     */
    async restoreCompany(id, { includeArchived = false } = {}) {
      try {
        const response = await apiRequest(`/companies/${id}/restore`, {
          method: 'POST',
        })
        if (response.ok) {
          await this.refresh(includeArchived)
          return { ok: true }
        }
        const error = await response.json().catch(() => ({}))
        return { ok: false, message: error.message }
      } catch (err) {
        console.error('Error restoring company:', err)
        return { ok: false, message: 'Ошибка сети' }
      }
    },
  },
})
