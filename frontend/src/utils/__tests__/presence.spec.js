import { describe, it, expect } from 'vitest'
import {
  ONLINE_WINDOW_MINUTES,
  isOnline,
  formatSeenShort,
  seenTitle,
  lastSeenSortKey,
} from '@/utils/presence'

const NOW = new Date('2026-07-30T12:00:00Z').getTime()

// Юзер с активностью N минут назад относительно NOW.
const seenMinutesAgo = (minutes, extra = {}) => ({
  username: 'u',
  is_active: true,
  is_banned: false,
  last_seen: new Date(NOW - minutes * 60_000).toISOString(),
  ...extra,
})

describe('presence — isOnline', () => {
  it('онлайн, пока активность внутри окна', () => {
    expect(isOnline(seenMinutesAgo(0), NOW)).toBe(true)
    expect(isOnline(seenMinutesAgo(ONLINE_WINDOW_MINUTES - 1), NOW)).toBe(true)
  })

  it('офлайн на границе окна и дальше', () => {
    expect(isOnline(seenMinutesAgo(ONLINE_WINDOW_MINUTES), NOW)).toBe(false)
    expect(isOnline(seenMinutesAgo(30), NOW)).toBe(false)
  })

  it('без last_seen офлайн', () => {
    expect(isOnline({ username: 'u', is_active: true, last_seen: null }, NOW)).toBe(false)
    expect(isOnline({ username: 'u', is_active: true }, NOW)).toBe(false)
    expect(isOnline(null, NOW)).toBe(false)
  })

  it('забаненный и архивный со свежей активностью не считаются онлайн', () => {
    expect(isOnline(seenMinutesAgo(1, { is_banned: true }), NOW)).toBe(false)
    expect(isOnline(seenMinutesAgo(1, { is_active: false }), NOW)).toBe(false)
  })

  it('битая дата не даёт ложный онлайн', () => {
    expect(isOnline({ is_active: true, last_seen: 'не дата' }, NOW)).toBe(false)
  })
})

describe('presence — formatSeenShort', () => {
  // Все ступени шкалы: секунды -> минуты -> часы -> дни -> месяцы -> годы, по две
  // старшие единицы. Подпись отвечает только за отсутствующих: присутствующим
  // ячейка рисует бейдж «Онлайн» (см. спеку UserControl.presence).
  const secondsAgo = (s) => ({ is_active: true, is_banned: true, last_seen: new Date(NOW - s * 1000).toISOString() })

  it('секунды и минуты с секундами', () => {
    expect(formatSeenShort(secondsAgo(0), NOW)).toBe('0 сек.')
    expect(formatSeenShort(secondsAgo(12), NOW)).toBe('12 сек.')
    expect(formatSeenShort(secondsAgo(59), NOW)).toBe('59 сек.')
    expect(formatSeenShort(secondsAgo(60), NOW)).toBe('1 мин.')
    expect(formatSeenShort(secondsAgo(200), NOW)).toBe('3 мин. 20 сек.')
  })

  it('вышедший за окно онлайна сразу показывает давность', () => {
    const justLeft = { is_active: true, last_seen: new Date(NOW - (ONLINE_WINDOW_MINUTES * 60 + 12) * 1000).toISOString() }
    expect(isOnline(justLeft, NOW)).toBe(false)
    expect(formatSeenShort(justLeft, NOW)).toBe('5 мин. 12 сек.')
  })

  it('часы с минутами, дни с часами', () => {
    expect(formatSeenShort(seenMinutesAgo(59), NOW)).toBe('59 мин.')
    expect(formatSeenShort(seenMinutesAgo(60), NOW)).toBe('1 ч.')
    expect(formatSeenShort(seenMinutesAgo(135), NOW)).toBe('2 ч. 15 мин.')
    expect(formatSeenShort(seenMinutesAgo(60 * 24), NOW)).toBe('1 дн.')
    expect(formatSeenShort(seenMinutesAgo(60 * 24 * 3 + 60 * 4), NOW)).toBe('3 дн. 4 ч.')
  })

  it('месяцы с днями и годы с месяцами', () => {
    const daysAgo = (d) => ({ is_active: true, last_seen: new Date(NOW - d * 86400_000).toISOString() })
    expect(formatSeenShort(daysAgo(30), NOW)).toBe('1 мес.')
    expect(formatSeenShort(daysAgo(30 * 5 + 12), NOW)).toBe('5 мес. 12 дн.')
    expect(formatSeenShort(daysAgo(365), NOW)).toBe('1 г.')
    expect(formatSeenShort(daysAgo(365 * 2 + 90), NOW)).toBe('2 г. 3 мес.')
  })

  it('нулевая младшая единица опускается', () => {
    expect(formatSeenShort(seenMinutesAgo(120), NOW)).toBe('2 ч.')
    expect(formatSeenShort(seenMinutesAgo(60 * 24 * 2), NOW)).toBe('2 дн.')
  })

  it('не заходившему рисует прочерк', () => {
    expect(formatSeenShort({ is_active: true, last_seen: null }, NOW)).toBe('-')
  })

  it('активность из будущего (перекос часов) читается как ноль', () => {
    const future = { is_active: true, is_banned: true, last_seen: new Date(NOW + 60_000).toISOString() }
    expect(formatSeenShort(future, NOW)).toBe('0 сек.')
  })
})

describe('presence — seenTitle', () => {
  it('онлайн, офлайн и «ни разу» формулируются по-разному', () => {
    const online = seenTitle(seenMinutesAgo(2), NOW)
    expect(online).toContain('В сети. Последняя активность: 2 мин. назад')
    const offline = seenTitle(seenMinutesAgo(90), NOW)
    expect(offline).toContain('Был в сети: 1 ч. 30 мин. назад')
    expect(seenTitle({ is_active: true, last_seen: null }, NOW)).toBe('Ни разу не заходил')
  })

  it('относительная часть подсказки совпадает с подписью ячейки', () => {
    const user = seenMinutesAgo(135)
    expect(seenTitle(user, NOW)).toContain(`${formatSeenShort(user, NOW)} назад`)
  })
})

describe('presence — lastSeenSortKey', () => {
  it('свежая активность больше старой, не заходившие уходят в конец по возрастанию', () => {
    const fresh = seenMinutesAgo(1)
    const old = seenMinutesAgo(500)
    const never = { is_active: true, last_seen: null }

    expect(lastSeenSortKey(fresh)).toBeGreaterThan(lastSeenSortKey(old))
    expect(lastSeenSortKey(never)).toBe(-Infinity)

    const sorted = [old, never, fresh].sort((a, b) => lastSeenSortKey(b) - lastSeenSortKey(a))
    expect(sorted).toEqual([fresh, old, never])
  })
})
