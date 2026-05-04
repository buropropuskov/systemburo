import { defineStore } from 'pinia'
import { apiRequest } from '@/api/client'

/**
 * Глобальный стор справочника гражданств.
 *
 * Использует один список { id, name, is_active, is_default, patent_required }
 * - в отличие от organizations/companies, у citizenships нет двух
 * представлений (нет endpoint'а с user_count).
 *
 * После CUD-операций через actions автоматически перечитываются данные -
 * любой компонент, читающий items через computed, получит свежий список.
 */
export const useCitizenshipsStore = defineStore('citizenships', {
  state: () => ({
    items: [],
    isLoading: false,
    error: null,
  }),

  getters: {
    findById: (state) => (id) => state.items.find(c => c.id === id) || null,
    defaultCitizenship: (state) => state.items.find(c => c.is_default) || null,
    activeItems: (state) => state.items.filter(c => c.is_active),
  },

  actions: {
    async fetchCitizenships() {
      try {
        const response = await apiRequest('/citizenships')
        if (response.ok) {
          this.items = await response.json()
        }
      } catch (err) {
        console.error('Error fetching citizenships:', err)
        this.error = err
      }
    },

    async refresh() {
      this.isLoading = true
      try {
        await this.fetchCitizenships()
      } finally {
        this.isLoading = false
      }
    },

    /**
     * @param {{ name: string, is_default?: boolean, patent_required?: boolean }} payload
     */
    async createCitizenship(payload) {
      try {
        const response = await apiRequest('/citizenships', {
          method: 'POST',
          body: JSON.stringify(payload),
        })
        if (response.ok) {
          await this.refresh()
          return { ok: true }
        }
        const message = await response.text()
        return { ok: false, message }
      } catch (err) {
        console.error('Error creating citizenship:', err)
        return { ok: false, message: 'Ошибка сети' }
      }
    },

    /**
     * @param {number} id
     * @param {{ name: string, is_active: boolean, is_default: boolean, patent_required: boolean }} payload
     */
    async updateCitizenship(id, payload) {
      try {
        const response = await apiRequest(`/citizenships/${id}`, {
          method: 'PUT',
          body: JSON.stringify(payload),
        })
        if (response.ok) {
          await this.refresh()
          return { ok: true }
        }
        const message = await response.text()
        return { ok: false, message }
      } catch (err) {
        console.error('Error updating citizenship:', err)
        return { ok: false, message: 'Ошибка сети' }
      }
    },

    /**
     * @param {number} id
     */
    async deleteCitizenship(id) {
      try {
        const response = await apiRequest(`/citizenships/${id}`, {
          method: 'DELETE',
        })
        if (response.ok) {
          await this.refresh()
          return { ok: true }
        }
        const error = await response.json().catch(() => ({}))
        return { ok: false, message: error.message }
      } catch (err) {
        console.error('Error deleting citizenship:', err)
        return { ok: false, message: 'Ошибка сети' }
      }
    },
  },
})
