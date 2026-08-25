import { describe, it, expect, beforeEach, vi } from 'vitest'

/**
 * Контракт обёртки с `client.js`: `apiRequest` возвращает Response, у которого
 * `.json()` уже развёрнут в `data`, а при отказе отдаёт `{message}` - и делает это
 * РЕЗОЛВОМ, не броском. Компонентный тест этого не стережёт: он мокает весь модуль
 * `@/api/applications` и проверяет поведение вокруг придуманного контракта (#1721).
 */

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
  apiRequestRaw: vi.fn(),
}))

import { apiRequest } from '@/api/client'
import { getApplicationParticipants } from '../applications'

function okJson(payload) {
  return { ok: true, status: 200, json: vi.fn().mockResolvedValue(payload) }
}

function errJson(message, status = 403) {
  return { ok: false, status, json: vi.fn().mockResolvedValue({ message }) }
}

describe('api/applications - участники заявки', () => {
  beforeEach(() => vi.clearAllMocks())

  it('GET /applications/:id/participants возвращает плоский массив', async () => {
    apiRequest.mockResolvedValue(okJson([{ user_id: 3, primary_role: 'approver' }]))

    const data = await getApplicationParticipants(7)

    expect(apiRequest).toHaveBeenCalledWith('/applications/7/participants')
    expect(data).toEqual([{ user_id: 3, primary_role: 'approver' }])
  })

  it('пустой ответ отдаёт массив, а не null - список рисуется без проверок на месте', async () => {
    apiRequest.mockResolvedValue(okJson(null))

    await expect(getApplicationParticipants(7)).resolves.toEqual([])
  })

  it('отказ бэка бросает его текстом: посторонний получает 403', async () => {
    apiRequest.mockResolvedValue(errJson('Нет доступа к заявке'))

    await expect(getApplicationParticipants(7)).rejects.toThrow('Нет доступа к заявке')
  })

  it('отказ без текста бросает понятное сообщение, а не undefined', async () => {
    apiRequest.mockResolvedValue({ ok: false, status: 500, json: vi.fn().mockResolvedValue(null) })

    await expect(getApplicationParticipants(7)).rejects.toThrow('Не удалось загрузить получателей заявки')
  })
})
