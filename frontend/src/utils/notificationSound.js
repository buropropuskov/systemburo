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
 * @param {'sine'|'triangle'|'square'|'sawtooth'} type - тип волны
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

  /** Три восходящих тона */
  chime(ctx, volume) {
    const t = ctx.currentTime
    playTone(ctx, 523, t, 0.2, volume * 0.8, 'sine')
    playTone(ctx, 659, t + 0.15, 0.2, volume * 0.9, 'sine')
    playTone(ctx, 784, t + 0.30, 0.35, volume, 'sine')
  },

  /** Низкий мягкий удар */
  thump(ctx, volume) {
    const t = ctx.currentTime
    playTone(ctx, 180, t, 0.25, volume, 'sine')
    playTone(ctx, 90, t + 0.05, 0.3, volume * 0.6, 'sine')
  },

  /** Быстрые три бипа */
  triple(ctx, volume) {
    const t = ctx.currentTime
    playTone(ctx, 1000, t, 0.1, volume, 'sine')
    playTone(ctx, 1000, t + 0.14, 0.1, volume, 'sine')
    playTone(ctx, 1200, t + 0.28, 0.18, volume, 'sine')
  },

  /** Нежный колокол */
  bell(ctx, volume) {
    const t = ctx.currentTime
    playTone(ctx, 1047, t, 0.6, volume, 'triangle')
    playTone(ctx, 2093, t, 0.3, volume * 0.3, 'triangle')
  },

  /** Восходящий свист */
  rise(ctx, volume) {
    const t = ctx.currentTime
    const osc = ctx.createOscillator()
    const gain = ctx.createGain()
    osc.connect(gain)
    gain.connect(ctx.destination)
    osc.type = 'sine'
    osc.frequency.setValueAtTime(300, t)
    osc.frequency.linearRampToValueAtTime(900, t + 0.3)
    gain.gain.setValueAtTime(0, t)
    gain.gain.linearRampToValueAtTime(volume, t + 0.05)
    gain.gain.exponentialRampToValueAtTime(0.001, t + 0.4)
    osc.start(t)
    osc.stop(t + 0.45)
  },

  /** Нисходящий свист */
  drop(ctx, volume) {
    const t = ctx.currentTime
    const osc = ctx.createOscillator()
    const gain = ctx.createGain()
    osc.connect(gain)
    gain.connect(ctx.destination)
    osc.type = 'sine'
    osc.frequency.setValueAtTime(900, t)
    osc.frequency.linearRampToValueAtTime(300, t + 0.3)
    gain.gain.setValueAtTime(0, t)
    gain.gain.linearRampToValueAtTime(volume, t + 0.05)
    gain.gain.exponentialRampToValueAtTime(0.001, t + 0.4)
    osc.start(t)
    osc.stop(t + 0.45)
  },

  /** Пиликание — два тона поочерёдно */
  ping(ctx, volume) {
    const t = ctx.currentTime
    playTone(ctx, 750, t, 0.12, volume, 'sine')
    playTone(ctx, 1000, t + 0.18, 0.18, volume, 'sine')
  },

  /** Мягкий треугольник — похож на «кухонный таймер» */
  timer(ctx, volume) {
    const t = ctx.currentTime
    playTone(ctx, 440, t, 0.5, volume, 'triangle')
    playTone(ctx, 550, t + 0.1, 0.35, volume * 0.5, 'triangle')
  },

  /** Короткий пульс два раза (квадратная волна) */
  pulse(ctx, volume) {
    const t = ctx.currentTime
    playTone(ctx, 660, t, 0.08, volume * 0.4, 'square')
    playTone(ctx, 660, t + 0.15, 0.08, volume * 0.4, 'square')
  },

  /** Фанфара — два восходящих аккорда */
  fanfare(ctx, volume) {
    const t = ctx.currentTime
    playTone(ctx, 523, t, 0.15, volume * 0.9, 'sine')
    playTone(ctx, 659, t, 0.15, volume * 0.7, 'sine')
    playTone(ctx, 784, t + 0.18, 0.25, volume, 'sine')
    playTone(ctx, 1047, t + 0.18, 0.25, volume * 0.6, 'sine')
  },
}

/**
 * Воспроизвести пресет.
 * @param {string} preset - ключ пресета
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
  { value: 'soft',    label: 'Мягкий' },
  { value: 'double',  label: 'Двойной' },
  { value: 'ding',    label: 'Динь' },
  { value: 'chime',   label: 'Перезвон' },
  { value: 'thump',   label: 'Глухой удар' },
  { value: 'triple',  label: 'Тройной' },
  { value: 'bell',    label: 'Колокол' },
  { value: 'rise',    label: 'Нарастающий' },
  { value: 'drop',    label: 'Нисходящий' },
  { value: 'ping',    label: 'Пинг' },
  { value: 'timer',   label: 'Таймер' },
  { value: 'pulse',   label: 'Импульс' },
  { value: 'fanfare', label: 'Фанфара' },
]
