import { describe, it, expect, beforeEach, vi } from 'vitest'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
}))
import { apiRequest } from '@/api/client'
import {
  canRevertMark,
  lastMarkDirection,
  markPassage,
  revertPassage,
  PASSAGE_REVERT_REASONS,
} from '../passageMarks'

function ok() {
  return { ok: true, status: 200, json: vi.fn().mockResolvedValue({}) }
}
function fail(message, status = 403) {
  return { ok: false, status, json: vi.fn().mockResolvedValue({ error: message }) }
}

describe('utils/passageMarks', () => {
  beforeEach(() => vi.clearAllMocks())

  describe('canRevertMark', () => {
    it('без признака с бэка кнопка отмены не появляется', () => {
      expect(canRevertMark({ can_revert: false, last_mark_table_id: 7 }, 7)).toBe(false)
      expect(canRevertMark({}, 7)).toBe(false)
      expect(canRevertMark(null, 7)).toBe(false)
    })

    it('отметка своего поста отменяется', () => {
      expect(canRevertMark({ can_revert: true, last_mark_table_id: 7 }, 7)).toBe(true)
    })

    it('отметка чужого поста не отменяется отсюда', () => {
      // Бэк на такой запрос ответит конфликтом, и кнопка-ловушка тут хуже её отсутствия.
      expect(canRevertMark({ can_revert: true, last_mark_table_id: 8 }, 7)).toBe(false)
    })

    it('без известного поста (легаси-записи) полагаемся на признак бэка', () => {
      expect(canRevertMark({ can_revert: true, last_mark_table_id: null }, 7)).toBe(true)
      expect(canRevertMark({ can_revert: true, last_mark_table_id: 8 }, null)).toBe(true)
    })
  })

  describe('lastMarkDirection', () => {
    it('статус строки задаёт направление последней отметки', () => {
      expect(lastMarkDirection({ territory_status: 1 })).toBe('entry')
      expect(lastMarkDirection({ territory_status: 2 })).toBe('exit')
      expect(lastMarkDirection({ territory_status: 0 })).toBeNull()
      expect(lastMarkDirection({})).toBeNull()
    })
  })

  describe('markPassage', () => {
    it('шлёт направление, автора и пост', async () => {
      apiRequest.mockResolvedValue(ok())
      await markPassage({ kind: 'employees', id: 5, direction: 'entry', userId: 3, tableId: 7 })

      const [url, options] = apiRequest.mock.calls[0]
      expect(url).toBe('/employees/5/territory-status')
      expect(options.method).toBe('PUT')
      expect(JSON.parse(options.body)).toEqual({ territory_status: 1, user_id: 3, table_id: 7 })
    })

    it('выезд «по факту» довозит данные пропуска', async () => {
      apiRequest.mockResolvedValue(ok())
      await markPassage({ kind: 'cars', id: 9, direction: 'exit', userId: 3, tableId: 7, pass: { number: 'А1' } })

      const [, options] = apiRequest.mock.calls[0]
      expect(JSON.parse(options.body)).toMatchObject({ territory_status: 2, pass: { number: 'А1' } })
    })
  })

  describe('revertPassage', () => {
    it('шлёт направление отменяемой отметки, пост и причину - и НЕ шлёт автора', async () => {
      apiRequest.mockResolvedValue(ok())
      const result = await revertPassage({
        kind: 'cars', id: 9, direction: 'entry', tableId: 7, reason: 'Отметил не того',
      })

      const [url, options] = apiRequest.mock.calls[0]
      expect(url).toBe('/cars/9/territory-status/revert')
      const body = JSON.parse(options.body)
      expect(body).toEqual({ territory_status: 1, table_id: 7, reason: 'Отметил не того' })
      // Автора ставит сервер из токена: пришли он телом, правило «своя отметка»
      // снималось бы подменой поля.
      expect(body.user_id).toBeUndefined()
      expect(result).toEqual({ ok: true, error: '' })
    })

    it('текст отказа берётся с бэка как есть', async () => {
      apiRequest.mockResolvedValue(fail('Отметку поставил другой пользователь'))
      const result = await revertPassage({ kind: 'cars', id: 9, direction: 'exit', tableId: 7, reason: 'ошибка' })

      expect(result.ok).toBe(false)
      expect(result.error).toBe('Отметку поставил другой пользователь')
    })

    it('неразобранное тело не оставляет человека без объяснения', async () => {
      apiRequest.mockResolvedValue({ ok: false, status: 500, json: vi.fn().mockRejectedValue(new Error('boom')) })
      const result = await revertPassage({ kind: 'cars', id: 9, direction: 'exit', tableId: 7, reason: 'ошибка' })

      expect(result).toEqual({ ok: false, error: 'Не удалось отменить отметку' })
    })
  })

  it('готовые причины непустые и короткие - на посту их читают на бегу', () => {
    expect(PASSAGE_REVERT_REASONS.length).toBeGreaterThan(0)
    PASSAGE_REVERT_REASONS.forEach((reason) => {
      expect(reason.trim()).not.toBe('')
      expect(reason.length).toBeLessThanOrEqual(40)
    })
  })
})
