import { defineStore } from 'pinia'
import { apiRequest } from '@/api/client'

/**
 * Глобальный стор справочника организаций.
 *
 * Используется компонентами ЛК (OrganizationsManagement, UserControl,
 * CreateUserModal). Поддерживает два представления:
 *   - items: лёгкий список { id, name } для dropdown'ов и выбора
 *   - itemsWithUsers: расширенный список с user_count и метаданными для
 *     управления (таблица в OrganizationsManagement)
 *
 * Любые CUD-операции выполняются через actions, что гарантирует, что все
 * подписчики (через computed на getters) получат свежие данные сразу - без
 * window-events и ручных fetch'ей в каждом компоненте.
 */
export const useOrganizationsStore = defineStore('organizations', {
  state: () => ({
    items: [],
    itemsWithUsers: [],
    isLoading: false,
    error: null,
  }),

  getters: {
    findById: (state) => (id) => state.items.find(o => o.id === id) || null,
    nameById: (state) => (id) => {
      const org = state.items.find(o => o.id === id)
      return org ? org.name : ''
    },
  },

  actions: {
    /**
     * Загружает базовый список { id, name } (используется UserControl/dropdown).
     */
    async fetchOrganizations() {
      try {
        const response = await apiRequest('/organizations')
        if (response.ok) {
          this.items = await response.json()
        }
      } catch (err) {
        console.error('Error fetching organizations:', err)
        this.error = err
      }
    },

    /**
     * Загружает расширенный список с user_count (для OrganizationsManagement).
     * Каждый элемент дополняется originalName для трекинга изменений inline.
     */
    async fetchOrganizationsWithUsers() {
      try {
        const response = await apiRequest('/organizations/with-users-extended')
        if (response.ok) {
          const data = await response.json()
          this.itemsWithUsers = data.map(org => ({
            ...org,
            originalName: org.name,
          }))
        }
      } catch (err) {
        console.error('Error fetching organizations with users:', err)
        this.error = err
      }
    },

    /**
     * Полная синхронизация: подтягивает оба представления параллельно.
     * Вызывается после CUD-операций, чтобы все потребители увидели свежие данные.
     */
    async refresh() {
      this.isLoading = true
      try {
        await Promise.all([
          this.fetchOrganizations(),
          this.fetchOrganizationsWithUsers(),
        ])
      } finally {
        this.isLoading = false
      }
    },

    /**
     * Создаёт организацию и обновляет state. Возвращает созданную сущность,
     * чтобы вызывающий мог автоматически выбрать её в UI.
     * @param {{ name: string }} payload
     * @returns {Promise<{ ok: boolean, data?: object, message?: string }>}
     */
    async createOrganization(payload) {
      try {
        const response = await apiRequest('/organizations', {
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
        console.error('Error creating organization:', err)
        return { ok: false, message: 'Ошибка сети' }
      }
    },

    /**
     * Обновляет организацию. После успеха перечитывает оба списка.
     * @param {number} id
     * @param {{ name: string }} payload
     */
    async updateOrganization(id, payload) {
      try {
        const response = await apiRequest(`/organizations/${id}`, {
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
        console.error('Error updating organization:', err)
        return { ok: false, message: 'Ошибка сети' }
      }
    },

    /**
     * Удаляет организацию. После успеха перечитывает оба списка.
     * @param {number} id
     */
    async deleteOrganization(id) {
      try {
        const response = await apiRequest(`/organizations/${id}`, {
          method: 'DELETE',
        })
        if (response.ok) {
          await this.refresh()
          return { ok: true }
        }
        const error = await response.json().catch(() => ({}))
        return { ok: false, message: error.message }
      } catch (err) {
        console.error('Error deleting organization:', err)
        return { ok: false, message: 'Ошибка сети' }
      }
    },
  },
})
