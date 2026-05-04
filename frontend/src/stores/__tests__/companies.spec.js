import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useCompaniesStore } from '../companies'

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

describe('companies store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  describe('fetchCompanies', () => {
    it('загружает базовый список из /companies', async () => {
      apiRequest.mockResolvedValue(okJson([{ id: 1, name: 'Acme Inc' }]))
      const store = useCompaniesStore()

      await store.fetchCompanies()

      expect(apiRequest).toHaveBeenCalledWith('/companies')
      expect(store.items).toEqual([{ id: 1, name: 'Acme Inc' }])
    })
  })

  describe('fetchCompaniesWithUsers', () => {
    it('добавляет originalName к каждому элементу', async () => {
      apiRequest.mockResolvedValue(okJson([{ id: 1, name: 'Acme Inc', user_count: 3 }]))
      const store = useCompaniesStore()

      await store.fetchCompaniesWithUsers()

      expect(store.itemsWithUsers).toEqual([
        { id: 1, name: 'Acme Inc', user_count: 3, originalName: 'Acme Inc' },
      ])
    })
  })

  describe('refresh', () => {
    it('загружает оба представления и сбрасывает isLoading', async () => {
      apiRequest
        .mockResolvedValueOnce(okJson([{ id: 1, name: 'A' }]))
        .mockResolvedValueOnce(okJson([{ id: 1, name: 'A', user_count: 0 }]))
      const store = useCompaniesStore()

      await store.refresh()

      expect(store.items).toHaveLength(1)
      expect(store.itemsWithUsers).toHaveLength(1)
      expect(store.isLoading).toBe(false)
    })
  })

  describe('createCompany', () => {
    it('создаёт компанию и подтягивает свежие данные', async () => {
      apiRequest
        .mockResolvedValueOnce(okJson({ id: 5, name: 'New Co' }))
        .mockResolvedValueOnce(okJson([{ id: 5, name: 'New Co' }]))
        .mockResolvedValueOnce(okJson([{ id: 5, name: 'New Co', user_count: 0 }]))

      const store = useCompaniesStore()
      const result = await store.createCompany({ name: 'New Co' })

      expect(result).toEqual({ ok: true, data: { id: 5, name: 'New Co' } })
      expect(store.items).toEqual([{ id: 5, name: 'New Co' }])
    })

    it('возвращает ошибку без рефреша при неуспехе', async () => {
      apiRequest.mockResolvedValueOnce(errJson({ message: 'fail' }))
      const store = useCompaniesStore()

      const result = await store.createCompany({ name: 'X' })

      expect(result).toEqual({ ok: false, message: 'fail' })
      expect(apiRequest).toHaveBeenCalledTimes(1)
    })
  })

  describe('updateCompany', () => {
    it('обновляет и рефрешит', async () => {
      apiRequest
        .mockResolvedValueOnce({ ok: true, json: vi.fn().mockResolvedValue({}) })
        .mockResolvedValueOnce(okJson([{ id: 1, name: 'Renamed' }]))
        .mockResolvedValueOnce(okJson([{ id: 1, name: 'Renamed', user_count: 0 }]))

      const store = useCompaniesStore()
      const result = await store.updateCompany(1, { name: 'Renamed' })

      expect(result.ok).toBe(true)
      expect(store.items[0].name).toBe('Renamed')
    })
  })

  describe('deleteCompany', () => {
    it('удаляет компанию', async () => {
      apiRequest
        .mockResolvedValueOnce({ ok: true, json: vi.fn().mockResolvedValue({}) })
        .mockResolvedValueOnce(okJson([]))
        .mockResolvedValueOnce(okJson([]))

      const store = useCompaniesStore()
      const result = await store.deleteCompany(1)

      expect(result.ok).toBe(true)
    })
  })

  describe('getters', () => {
    it('findById и nameById', () => {
      const store = useCompaniesStore()
      store.items = [{ id: 1, name: 'A' }]

      expect(store.findById(1)).toEqual({ id: 1, name: 'A' })
      expect(store.findById(2)).toBeNull()
      expect(store.nameById(1)).toBe('A')
      expect(store.nameById(2)).toBe('')
    })
  })
})
