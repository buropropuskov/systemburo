import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useOrganizationsStore } from '../organizations'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
}))
import { apiRequest } from '@/api/client'

function okJson(payload) {
  return {
    ok: true,
    json: vi.fn().mockResolvedValue(payload),
  }
}

function errJson(payload, status = 400) {
  return {
    ok: false,
    status,
    json: vi.fn().mockResolvedValue(payload),
  }
}

describe('organizations store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  describe('fetchOrganizations', () => {
    it('загружает базовый список из /organizations', async () => {
      apiRequest.mockResolvedValue(okJson([{ id: 1, name: 'Acme' }, { id: 2, name: 'Globex' }]))
      const store = useOrganizationsStore()

      await store.fetchOrganizations()

      expect(apiRequest).toHaveBeenCalledWith('/organizations')
      expect(store.items).toEqual([{ id: 1, name: 'Acme' }, { id: 2, name: 'Globex' }])
    })

    it('не падает при сетевой ошибке', async () => {
      apiRequest.mockRejectedValue(new Error('network'))
      const store = useOrganizationsStore()

      await store.fetchOrganizations()

      expect(store.items).toEqual([])
      expect(store.error).toBeInstanceOf(Error)
    })
  })

  describe('fetchOrganizationsWithUsers', () => {
    it('добавляет originalName к каждому элементу', async () => {
      apiRequest.mockResolvedValue(okJson([{ id: 1, name: 'Acme', user_count: 5 }]))
      const store = useOrganizationsStore()

      await store.fetchOrganizationsWithUsers()

      // silent403 обязателен: список закрыт правом справочников, а зовёт его и
      // UserControl - без тишины админ пользователей ловил бы тост на ровном месте.
      expect(apiRequest).toHaveBeenCalledWith('/organizations/with-users-extended', { silent403: true })
      expect(store.itemsWithUsers).toEqual([
        { id: 1, name: 'Acme', user_count: 5, originalName: 'Acme' },
      ])
    })

    it('добавляет include_archived при includeArchived=true', async () => {
      apiRequest.mockResolvedValue(okJson([{ id: 1, name: 'Acme', user_count: 5, is_active: false }]))
      const store = useOrganizationsStore()

      await store.fetchOrganizationsWithUsers(true)

      expect(apiRequest).toHaveBeenCalledWith('/organizations/with-users-extended?include_archived=true', { silent403: true })
    })
  })

  describe('refresh', () => {
    it('подтягивает оба представления параллельно', async () => {
      apiRequest
        .mockResolvedValueOnce(okJson([{ id: 1, name: 'Acme' }]))
        .mockResolvedValueOnce(okJson([{ id: 1, name: 'Acme', user_count: 0 }]))
      const store = useOrganizationsStore()

      await store.refresh()

      expect(apiRequest).toHaveBeenCalledTimes(2)
      expect(store.items).toHaveLength(1)
      expect(store.itemsWithUsers).toHaveLength(1)
      expect(store.isLoading).toBe(false)
    })
  })

  describe('createOrganization', () => {
    it('создаёт организацию и обновляет state через refresh', async () => {
      apiRequest
        .mockResolvedValueOnce(okJson({ id: 7, name: 'New' }))
        .mockResolvedValueOnce(okJson([{ id: 7, name: 'New' }]))
        .mockResolvedValueOnce(okJson([{ id: 7, name: 'New', user_count: 0 }]))

      const store = useOrganizationsStore()
      const result = await store.createOrganization({ name: 'New' })

      expect(result.ok).toBe(true)
      expect(result.data).toEqual({ id: 7, name: 'New' })
      expect(store.items).toEqual([{ id: 7, name: 'New' }])
      expect(store.itemsWithUsers[0]).toMatchObject({ id: 7, name: 'New', originalName: 'New' })
    })

    it('возвращает ошибку при неуспешном ответе', async () => {
      apiRequest.mockResolvedValueOnce(errJson({ message: 'duplicate' }))
      const store = useOrganizationsStore()

      const result = await store.createOrganization({ name: 'dup' })

      expect(result).toEqual({ ok: false, message: 'duplicate' })
      // refresh не должен быть вызван
      expect(apiRequest).toHaveBeenCalledTimes(1)
    })

    it('возвращает сетевую ошибку при reject', async () => {
      apiRequest.mockRejectedValueOnce(new Error('fail'))
      const store = useOrganizationsStore()

      const result = await store.createOrganization({ name: 'x' })

      expect(result).toEqual({ ok: false, message: 'Ошибка сети' })
    })
  })

  describe('updateOrganization', () => {
    it('обновляет организацию и подтягивает refresh', async () => {
      apiRequest
        .mockResolvedValueOnce({ ok: true, json: vi.fn().mockResolvedValue({}) })
        .mockResolvedValueOnce(okJson([{ id: 1, name: 'Renamed' }]))
        .mockResolvedValueOnce(okJson([{ id: 1, name: 'Renamed', user_count: 2 }]))

      const store = useOrganizationsStore()
      const result = await store.updateOrganization(1, { name: 'Renamed' })

      expect(result.ok).toBe(true)
      expect(apiRequest).toHaveBeenNthCalledWith(1, '/organizations/1', {
        method: 'PUT',
        body: JSON.stringify({ name: 'Renamed' }),
      })
      expect(store.items[0].name).toBe('Renamed')
    })
  })

  describe('deleteOrganization', () => {
    it('удаляет и обновляет state', async () => {
      apiRequest
        .mockResolvedValueOnce({ ok: true, json: vi.fn().mockResolvedValue({}) })
        .mockResolvedValueOnce(okJson([]))
        .mockResolvedValueOnce(okJson([]))

      const store = useOrganizationsStore()
      const result = await store.deleteOrganization(1)

      expect(result.ok).toBe(true)
      expect(apiRequest).toHaveBeenNthCalledWith(1, '/organizations/1', { method: 'DELETE' })
      expect(store.items).toEqual([])
    })
  })

  describe('restoreOrganization', () => {
    it('восстанавливает из архива и обновляет state', async () => {
      apiRequest
        .mockResolvedValueOnce({ ok: true, json: vi.fn().mockResolvedValue({}) })
        .mockResolvedValueOnce(okJson([{ id: 1, name: 'Back' }]))
        .mockResolvedValueOnce(okJson([{ id: 1, name: 'Back', user_count: 0 }]))

      const store = useOrganizationsStore()
      const result = await store.restoreOrganization(1, { includeArchived: true })

      expect(result.ok).toBe(true)
      expect(apiRequest).toHaveBeenNthCalledWith(1, '/organizations/1/restore', { method: 'POST' })
      expect(store.items[0].name).toBe('Back')
    })

    it('возвращает ошибку при неуспешном ответе', async () => {
      apiRequest.mockResolvedValueOnce(errJson({ message: 'нельзя' }, 409))
      const store = useOrganizationsStore()

      const result = await store.restoreOrganization(1)

      expect(result).toEqual({ ok: false, message: 'нельзя' })
      expect(apiRequest).toHaveBeenCalledTimes(1)
    })
  })

  describe('getters', () => {
    it('findById возвращает организацию по id', () => {
      const store = useOrganizationsStore()
      store.items = [{ id: 1, name: 'A' }, { id: 2, name: 'B' }]

      expect(store.findById(2)).toEqual({ id: 2, name: 'B' })
      expect(store.findById(99)).toBeNull()
    })

    it('nameById возвращает имя или пустую строку', () => {
      const store = useOrganizationsStore()
      store.items = [{ id: 1, name: 'A' }]

      expect(store.nameById(1)).toBe('A')
      expect(store.nameById(99)).toBe('')
    })
  })
})
