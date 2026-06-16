/**
 * Синтез коротких звуковых пресетов через Web Audio API.
 * Не требует бинарных файлов — тоны генерируются программно.
 *
 * Autoplay-политика: первый AudioContext до user-gesture браузер может заблокировать.
 * Ошибки намеренно подавляются (best-effort) — приложение не должно ломаться из-за звука.
 */

let _ctx = null

function getAudioContext() {
  if (!_ctx) {
    try {
      _ctx = new (window.AudioContext || window.webkitAudioContext)()
    } catch {
      return null
    }
  }
  // Некоторые браузеры suspend'ят контекст до первого user-gesture
  if (_ctx.state === 'suspended') {
    _ctx.resume().catch(() => {})
  }
  return _ctx
}

/**
 * Воспроизвести один тон заданной частоты и длительности.
 * @param {AudioContext} ctx
 * @param {number} freq - частота в Гц
 * @param {number} startAt - время начала в секундах (ctx.currentTime-базированное)
 * @param {number} duration - длительность тона в секундах
 * @param {number} volume - громкость 0..1
 * @param {'sine'|'triangle'} type - тип волны
 */
function playTone(ctx, freq, startAt, duration, volume, type = 'sine') {
  const osc = ctx.createOscillator()
  const gain = ctx.createGain()

  osc.connect(gain)
  gain.connect(ctx.destination)

  osc.type = type
  osc.frequency.setValueAtTime(freq, startAt)

  gain.gain.setValueAtTime(0, startAt)
  gain.gain.linearRampToValueAtTime(volume, startAt + 0.01)
  gain.gain.exponentialRampToValueAtTime(0.001, startAt + duration)

  osc.start(startAt)
  osc.stop(startAt + duration + 0.05)
}

const PRESETS = {
  /** Один мягкий тон ~600 Гц */
  soft(ctx, volume) {
    const t = ctx.currentTime
    playTone(ctx, 600, t, 0.4, volume, 'sine')
  },

  /** Два коротких бипа */
  double(ctx, volume) {
    const t = ctx.currentTime
    playTone(ctx, 880, t, 0.15, volume, 'sine')
    playTone(ctx, 880, t + 0.22, 0.15, volume, 'sine')
  },

  /** Высокий короткий «динь» */
  ding(ctx, volume) {
    const t = ctx.currentTime
    playTone(ctx, 1320, t, 0.3, volume, 'triangle')
  },
}

/**
 * Воспроизвести пресет.
 * @param {'soft'|'double'|'ding'} preset
 * @param {number} volume - 0..1
 */
export function playPreset(preset, volume) {
  try {
    const ctx = getAudioContext()
    if (!ctx) return
    const fn = PRESETS[preset] || PRESETS.soft
    fn(ctx, Math.max(0, Math.min(1, volume)))
  } catch {
    // autoplay-блок или нет Web Audio — не критично
  }
}

export const SOUND_PRESETS = [
  { value: 'soft',   label: 'Мягкий' },
  { value: 'double', label: 'Двойной' },
  { value: 'ding',   label: 'Динь' },
]
