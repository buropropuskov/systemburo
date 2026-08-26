import { describe, it, expect, beforeEach, vi } from 'vitest'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
}))
import { apiRequest } from '@/api/client'
import {
  getApprovers,
  getAllUsers,
  addApprover,
  updateApprover,
  deleteApprover,
  getApproverHistory,
} from '../approvers'

function okJson(payload) {
  return { ok: true, status: 200, json: vi.fn().mockResolvedValue(payload) }
}
function errJson(message, status = 400) {
  return { ok: false, status, json: vi.fn().mockResolvedValue({ message }) }
}

describe('api/approvers', () => {
  beforeEach(() => vi.clearAllMocks())

  describe('getApprovers', () => {
    it('GET /application-approvers возвращает массив', async () => {
      apiRequest.mockResolvedValue(okJson([{ id: 1, user_id: 5 }]))
      const data = await getApprovers()
      expect(apiRequest).toHaveBeenCalledWith('/application-approvers')
      expect(data).toEqual([{ id: 1, user_id: 5 }])
    })

    it('бросает при ошибке загрузки', async () => {
      apiRequest.mockResolvedValue(errJson('Сервер недоступен', 500))
      await expect(getApprovers()).rejects.toThrow('Сервер недоступен')
    })
  })

  describe('getAllUsers', () => {
    it('GET /users/all возвращает массив', async () => {
      apiRequest.mockResolvedValue(okJson([{ id: 2, username: 'ivan' }]))
      const data = await getAllUsers()
      expect(apiRequest).toHaveBeenCalledWith('/users/all')
      expect(data).toEqual([{ id: 2, username: 'ivan' }])
    })

    it('бросает при ошибке загрузки', async () => {
      apiRequest.mockResolvedValue(errJson('Нет доступа', 403))
      await expect(getAllUsers()).rejects.toThrow('Нет доступа')
    })
  })

  describe('addApprover', () => {
    it('POST /application-approvers с user_id (201)', async () => {
      apiRequest.mockResolvedValue({ ok: true, status: 201, json: vi.fn().mockResolvedValue({ id: 3, user_id: 7 }) })
      await addApprover(7)
      expect(apiRequest).toHaveBeenCalledWith('/application-approvers', {
        method: 'POST',
        body: JSON.stringify({ user_id: 7 }),
      })
    })

    it('бросает при ошибке добавления', async () => {
      apiRequest.mockResolvedValue(errJson('Пользователь уже является принимающим', 409))
      await expect(addApprover(7)).rejects.toThrow('Пользователь уже является принимающим')
    })
  })

  describe('updateApprover', () => {
    it('PATCH /application-approvers/{id} с маской', async () => {
      apiRequest.mockResolvedValue(okJson({ message: 'ok' }))
      await updateApprover(3, 'Оператор Бюро')
      expect(apiRequest).toHaveBeenCalledWith('/application-approvers/3', {
        method: 'PATCH',
        body: JSON.stringify({ display_name: 'Оператор Бюро' }),
      })
    })

    it('PATCH с null снимает маску', async () => {
      apiRequest.mockResolvedValue(okJson({ message: 'ok' }))
      await updateApprover(3, null)
      expect(apiRequest).toHaveBeenCalledWith('/application-approvers/3', {
        method: 'PATCH',
        body: JSON.stringify({ display_name: null }),
      })
    })

    it('бросает при ошибке сохранения', async () => {
      apiRequest.mockResolvedValue(errJson('Принимающий не найден', 404))
      await expect(updateApprover(3, 'X')).rejects.toThrow('Принимающий не найден')
    })
  })

  describe('deleteApprover', () => {
    it('DELETE /application-approvers/{id}', async () => {
      apiRequest.mockResolvedValue(okJson('ok'))
      await deleteApprover(3)
      expect(apiRequest).toHaveBeenCalledWith('/application-approvers/3', { method: 'DELETE' })
    })

    it('бросает при ошибке удаления', async () => {
      apiRequest.mockResolvedValue(errJson('Принимающий не найден', 404))
      await expect(deleteApprover(3)).rejects.toThrow('Принимающий не найден')
    })
  })

  describe('getApproverHistory', () => {
    it('GET /application-approvers/history (глобальный, без id) возвращает массив', async () => {
      const log = [{ id: 1, approver_user_id: 5, action_type: 'created' }]
      apiRequest.mockResolvedValue(okJson(log))
      const data = await getApproverHistory()
      expect(apiRequest).toHaveBeenCalledWith('/application-approvers/history')
      expect(data).toEqual(log)
    })

    it('бросает при ошибке загрузки', async () => {
      apiRequest.mockResolvedValue(errJson('Нет доступа', 403))
      await expect(getApproverHistory()).rejects.toThrow('Нет доступа')
    })
  })
})
