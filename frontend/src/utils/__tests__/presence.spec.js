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
  it('внутри окна показывает «в сети»', () => {
    expect(formatSeenShort(seenMinutesAgo(2), NOW)).toBe('в сети')
  })

  it('минуты, часы и дни за пределами окна', () => {
    expect(formatSeenShort(seenMinutesAgo(12), NOW)).toBe('12 мин')
    expect(formatSeenShort(seenMinutesAgo(59), NOW)).toBe('59 мин')
    expect(formatSeenShort(seenMinutesAgo(60), NOW)).toBe('1 ч')
    expect(formatSeenShort(seenMinutesAgo(60 * 23), NOW)).toBe('23 ч')
    expect(formatSeenShort(seenMinutesAgo(60 * 24), NOW)).toBe('1 дн')
    expect(formatSeenShort(seenMinutesAgo(60 * 24 * 3 + 5), NOW)).toBe('3 дн')
  })

  it('не заходившему рисует прочерк', () => {
    expect(formatSeenShort({ is_active: true, last_seen: null }, NOW)).toBe('-')
  })

  it('забаненный со свежей активностью показывает время, а не «в сети»', () => {
    expect(formatSeenShort(seenMinutesAgo(2, { is_banned: true }), NOW)).toBe('2 мин')
  })

  it('активность из будущего (перекос часов) не даёт «0 мин»', () => {
    const future = { is_active: true, is_banned: true, last_seen: new Date(NOW + 60_000).toISOString() }
    expect(formatSeenShort(future, NOW)).toBe('1 мин')
  })
})

describe('presence — seenTitle', () => {
  it('онлайн, офлайн и «ни разу» формулируются по-разному', () => {
    expect(seenTitle(seenMinutesAgo(2), NOW)).toContain('В сети')
    const offline = seenTitle(seenMinutesAgo(90), NOW)
    expect(offline).toContain('Был в сети')
    expect(offline).toContain('1 ч назад')
    expect(seenTitle({ is_active: true, last_seen: null }, NOW)).toBe('Ни разу не заходил')
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
