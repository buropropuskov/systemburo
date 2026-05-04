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

    async fetchCompaniesWithUsers() {
      try {
        const response = await apiRequest('/companies/with-users-extended')
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

    async refresh() {
      this.isLoading = true
      try {
        await Promise.all([
          this.fetchCompanies(),
          this.fetchCompaniesWithUsers(),
        ])
      } finally {
        this.isLoading = false
      }
    },

    /**
     * @param {{ name: string }} payload
     */
    async createCompany(payload) {
      try {
        const response = await apiRequest('/companies', {
          method: 'POST',
          body: JSON.stringify(payload),
        })
        if (response.ok) {
          const data = await response.json()
          await this.refresh()
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
     */
    async updateCompany(id, payload) {
      try {
        const response = await apiRequest(`/companies/${id}`, {
          method: 'PUT',
          body: JSON.stringify(payload),
        })
        if (response.ok) {
          await this.refresh()
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
     * @param {number} id
     */
    async deleteCompany(id) {
      try {
        const response = await apiRequest(`/companies/${id}`, {
          method: 'DELETE',
        })
        if (response.ok) {
          await this.refresh()
          return { ok: true }
        }
        const error = await response.json().catch(() => ({}))
        return { ok: false, message: error.message }
      } catch (err) {
        console.error('Error deleting company:', err)
        return { ok: false, message: 'Ошибка сети' }
      }
    },
  },
})
