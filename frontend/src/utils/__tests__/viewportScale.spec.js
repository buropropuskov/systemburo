import { describe, it, expect } from 'vitest'
import { computeZoom } from '../viewportScale'

describe('viewportScale computeZoom', () => {
  it('не масштабирует на эталоне и уже (обычная адаптивная вёрстка)', () => {
    expect(computeZoom(1440)).toBe(1)
    expect(computeZoom(1280)).toBe(1)
    expect(computeZoom(1024)).toBe(1)
    expect(computeZoom(768)).toBe(1)
    expect(computeZoom(390)).toBe(1)
  })

  it('масштабирует широкие экраны под эталон 1440', () => {
    // 1920@100% должен выглядеть как 1440@100%
    expect(computeZoom(1920)).toBeCloseTo(1920 / 1440, 4)
    // ключевой кейс пользователя: 2560@100% = как 1440@100%
    expect(computeZoom(2560)).toBeCloseTo(2560 / 1440, 4)
  })

  it('упирается в потолок на ультрашироких/мультимониторе (не раздуваем)', () => {
    expect(computeZoom(2880)).toBe(2) // ровно 2x1440
    expect(computeZoom(3440)).toBe(2) // ultrawide
    expect(computeZoom(5120)).toBe(2) // два монитора
  })
})
