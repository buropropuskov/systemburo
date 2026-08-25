import { defineStore } from 'pinia'
import { apiRequest } from '@/api/client'

/**
 * Глобальный стор справочника организаций.
 *
 * Используется компонентами ЛК (OrganizationsManagement, UserControl).
 * Поддерживает два представления:
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
     *
     * Список закрыт правом page.admin.directories (#2002), а зовут его и из
     * UserControl - там он держит user_count свежим для соседнего экрана. Админ
     * пользователей без права справочников получает на нём штатный 403, и тост о
     * нём был бы про действие, которого человек не совершал: он всего лишь завёл
     * пользователя. Отказ здесь молчит, экран управления пользователями работает
     * без этих данных.
     * @param {boolean} includeArchived - включить архивные организации (is_active=false)
     */
    async fetchOrganizationsWithUsers(includeArchived = false) {
      try {
        const qs = includeArchived ? '?include_archived=true' : ''
        const response = await apiRequest(`/organizations/with-users-extended${qs}`, { silent403: true })
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
     * @param {boolean} includeArchived - тянуть ли архивные в расширенный список
     */
    async refresh(includeArchived = false) {
      this.isLoading = true
      try {
        await Promise.all([
          this.fetchOrganizations(),
          this.fetchOrganizationsWithUsers(includeArchived),
        ])
      } finally {
        this.isLoading = false
      }
    },

    /**
     * Создаёт организацию и обновляет state. Возвращает созданную сущность,
     * чтобы вызывающий мог автоматически выбрать её в UI.
     * @param {{ name: string }} payload
     * @param {{ includeArchived?: boolean }} [opts] - сохранить режим архива при рефреше
     * @returns {Promise<{ ok: boolean, data?: object, message?: string }>}
     */
    async createOrganization(payload, { includeArchived = false } = {}) {
      try {
        const response = await apiRequest('/organizations', {
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
        console.error('Error creating organization:', err)
        return { ok: false, message: 'Ошибка сети' }
      }
    },

    /**
     * Обновляет организацию. После успеха перечитывает оба списка.
     * @param {number} id
     * @param {{ name: string }} payload
     * @param {{ includeArchived?: boolean }} [opts]
     */
    async updateOrganization(id, payload, { includeArchived = false } = {}) {
      try {
        const response = await apiRequest(`/organizations/${id}`, {
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
        console.error('Error updating organization:', err)
        return { ok: false, message: 'Ошибка сети' }
      }
    },

    /**
     * Архивирует организацию (soft-delete, is_active=false). После успеха перечитывает списки.
     * @param {number} id
     * @param {{ includeArchived?: boolean }} [opts]
     */
    async deleteOrganization(id, { includeArchived = false } = {}) {
      try {
        const response = await apiRequest(`/organizations/${id}`, {
          method: 'DELETE',
        })
        if (response.ok) {
          await this.refresh(includeArchived)
          return { ok: true }
        }
        const error = await response.json().catch(() => ({}))
        return { ok: false, message: error.message }
      } catch (err) {
        console.error('Error deleting organization:', err)
        return { ok: false, message: 'Ошибка сети' }
      }
    },

    /**
     * Восстанавливает организацию из архива (is_active=true). После успеха перечитывает списки.
     * @param {number} id
     * @param {{ includeArchived?: boolean }} [opts]
     */
    async restoreOrganization(id, { includeArchived = false } = {}) {
      try {
        const response = await apiRequest(`/organizations/${id}/restore`, {
          method: 'POST',
        })
        if (response.ok) {
          await this.refresh(includeArchived)
          return { ok: true }
        }
        const error = await response.json().catch(() => ({}))
        return { ok: false, message: error.message }
      } catch (err) {
        console.error('Error restoring organization:', err)
        return { ok: false, message: 'Ошибка сети' }
      }
    },
  },
})
