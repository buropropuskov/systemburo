import { describe, it, expect } from 'vitest'
import { computeZoom, updateViewportZoom } from '../viewportScale'

describe('viewportScale computeZoom', () => {
  it('не масштабирует на эталоне и уже (обычная адаптивная вёрстка)', () => {
    expect(computeZoom(1440, 900)).toBe(1)
    expect(computeZoom(1280, 800)).toBe(1)
    expect(computeZoom(1024, 768)).toBe(1)
    expect(computeZoom(768, 1024)).toBe(1)
    expect(computeZoom(390, 844)).toBe(1)
  })

  it('масштабирует широкие экраны, ограничивая по высоте', () => {
    // ключевой кейс пользователя: 2560x1440 -> ограничивает высота (900), zoom 1.6
    expect(computeZoom(2560, 1440)).toBeCloseTo(1.6, 4)
    // 1920x1080 -> высота (1080/900=1.2) уже ширины (1.333), берём 1.2
    expect(computeZoom(1920, 1080)).toBeCloseTo(1.2, 4)
    // высокий экран: 2560x1600 -> ширина (1.778) уже высоты (1.778), берём 1.778
    expect(computeZoom(2560, 1600)).toBeCloseTo(2560 / 1440, 4)
  })

  it('низкий широкий экран не сжимаем ниже 1', () => {
    // 2560x800: по высоте 0.889 -> зажали бы UI, оставляем 1
    expect(computeZoom(2560, 800)).toBe(1)
  })

  it('упирается в потолок на ультрашироких/мультимониторе (не раздуваем)', () => {
    expect(computeZoom(2880, 2000)).toBe(2) // ширина ровно 2x, высота больше
    expect(computeZoom(5120, 4000)).toBe(2) // два монитора
  })
})

describe('viewportScale updateViewportZoom (DOM-эффект)', () => {
  // Один последовательный сценарий: модуль стартует с appliedZoom=1, ветки
  // set/clear/threshold зависят от предыдущего состояния, поэтому идём по порядку.
  it('ставит и сбрасывает style.zoom по размеру окна', () => {
    const root = document.documentElement

    window.innerWidth = 2560
    window.innerHeight = 1440
    updateViewportZoom()
    expect(root.style.zoom).toBe('1.6') // 2560x1440 -> ограничено высотой
    // --app-vh = зумленная высота вьюпорта / 100 = 1440/1.6/100 = 9px
    expect(root.style.getPropertyValue('--app-vh')).toBe('9px')

    window.innerWidth = 1440
    window.innerHeight = 900
    updateViewportZoom()
    expect(root.style.zoom).toBe('') // вернулись к эталону -> zoom снят

    window.innerWidth = 1920
    window.innerHeight = 1080
    updateViewportZoom()
    expect(root.style.zoom).toBe('1.2') // другой широкий -> пересчёт

    root.style.zoom = ''
  })
})
