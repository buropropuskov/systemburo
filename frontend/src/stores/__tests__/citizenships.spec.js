import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useCitizenshipsStore } from '../citizenships'

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

function errText(text, status = 400) {
  return {
    ok: false,
    status,
    text: vi.fn().mockResolvedValue(text),
  }
}

describe('citizenships store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  describe('fetchCitizenships', () => {
    it('загружает справочник из /citizenships', async () => {
      apiRequest.mockResolvedValue(okJson([
        { id: 1, name: 'РФ', is_active: true, is_default: true, patent_required: false },
      ]))
      const store = useCitizenshipsStore()

      await store.fetchCitizenships()

      expect(apiRequest).toHaveBeenCalledWith('/citizenships')
      expect(store.items).toHaveLength(1)
    })
  })

  describe('createCitizenship', () => {
    it('создаёт и подтягивает свежий список', async () => {
      apiRequest
        .mockResolvedValueOnce({ ok: true, json: vi.fn().mockResolvedValue({}) })
        .mockResolvedValueOnce(okJson([{ id: 2, name: 'KZ', is_active: true, is_default: false, patent_required: true }]))

      const store = useCitizenshipsStore()
      const result = await store.createCitizenship({ name: 'KZ' })

      expect(result).toEqual({ ok: true })
      expect(store.items).toHaveLength(1)
      expect(store.items[0].name).toBe('KZ')
    })

    it('возвращает текстовое сообщение при ошибке', async () => {
      apiRequest.mockResolvedValueOnce(errText('Already exists'))
      const store = useCitizenshipsStore()

      const result = await store.createCitizenship({ name: 'dup' })

      expect(result).toEqual({ ok: false, message: 'Already exists' })
    })
  })

  describe('updateCitizenship', () => {
    it('обновляет гражданство и рефрешит state', async () => {
      apiRequest
        .mockResolvedValueOnce({ ok: true, json: vi.fn().mockResolvedValue({}) })
        .mockResolvedValueOnce(okJson([
          { id: 1, name: 'РФ updated', is_active: true, is_default: false, patent_required: false },
        ]))

      const store = useCitizenshipsStore()
      const result = await store.updateCitizenship(1, {
        name: 'РФ updated',
        is_active: true,
        is_default: false,
        patent_required: false,
      })

      expect(result.ok).toBe(true)
      expect(store.items[0].name).toBe('РФ updated')
    })
  })

  describe('deleteCitizenship', () => {
    it('удаляет и обновляет state', async () => {
      apiRequest
        .mockResolvedValueOnce({ ok: true, json: vi.fn().mockResolvedValue({}) })
        .mockResolvedValueOnce(okJson([]))

      const store = useCitizenshipsStore()
      const result = await store.deleteCitizenship(1)

      expect(result.ok).toBe(true)
      expect(store.items).toEqual([])
    })
  })

  describe('getters', () => {
    it('defaultCitizenship возвращает is_default=true', () => {
      const store = useCitizenshipsStore()
      store.items = [
        { id: 1, name: 'РФ', is_active: true, is_default: false, patent_required: false },
        { id: 2, name: 'KZ', is_active: true, is_default: true, patent_required: false },
      ]

      expect(store.defaultCitizenship).toEqual(store.items[1])
    })

    it('activeItems фильтрует по is_active', () => {
      const store = useCitizenshipsStore()
      store.items = [
        { id: 1, name: 'A', is_active: true, is_default: false, patent_required: false },
        { id: 2, name: 'B', is_active: false, is_default: false, patent_required: false },
      ]

      expect(store.activeItems).toHaveLength(1)
      expect(store.activeItems[0].id).toBe(1)
    })

    it('findById возвращает гражданство или null', () => {
      const store = useCitizenshipsStore()
      store.items = [{ id: 5, name: 'X' }]

      expect(store.findById(5)).toEqual({ id: 5, name: 'X' })
      expect(store.findById(99)).toBeNull()
    })
  })
})
