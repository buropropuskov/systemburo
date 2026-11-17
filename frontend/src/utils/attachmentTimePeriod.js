import { toMinutes } from './timeSlots.js'

const DAY_START = 8 * 60
const NIGHT_START = 22 * 60
const MINUTES_PER_DAY = 24 * 60
const CLOCK_TIME = /^(?:[01]\d|2[0-3]):[0-5]\d(?::[0-5]\d)?$/

export function getAttachmentTimePeriod(timeFrom, timeTo) {
  if (typeof timeFrom !== 'string' || typeof timeTo !== 'string' ||
      !CLOCK_TIME.test(timeFrom) || !CLOCK_TIME.test(timeTo)) return null

  const start = toMinutes(timeFrom) + Number(timeFrom.slice(6)) / 60
  const end = toMinutes(timeTo) + Number(timeTo.slice(6)) / 60
  if (start === end) return null

  const ranges = end < start ? [[start, MINUTES_PER_DAY], [0, end]] : [[start, end]]
  const touchesDay = ranges.some(([from, to]) => from < to && from < NIGHT_START && to > DAY_START)
  const touchesNight = ranges.some(([from, to]) => from < to && (from < DAY_START || to > NIGHT_START))

  if (touchesDay && touchesNight) return 'mixed'
  return touchesDay ? 'day' : 'night'
}
