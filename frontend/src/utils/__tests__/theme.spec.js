import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, it, expect, beforeEach, vi } from 'vitest'
import {
  THEMES,
  THEME_STORAGE_KEY,
  DEFAULT_THEME,
  isValidTheme,
  findTheme,
  applyTheme,
  readStoredTheme,
  storeTheme,
} from '@/utils/theme'

describe('utils/theme', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.removeAttribute('data-theme')
  })

  it('реестр содержит шесть тем с уникальными id', () => {
    const ids = THEMES.map((t) => t.id)
    expect(ids).toEqual([
      'light',
      'dark',
      'corporate-orange',
      'business-graphite',
      'official-blue',
      'dark-orange',
    ])
    expect(new Set(ids).size).toBe(ids.length)
    THEMES.forEach((t) => {
      expect(t.name).toBeTruthy()
      expect(t.dot).toMatch(/^#[0-9a-fA-F]{6}$/)
    })
  })

  it('isValidTheme пропускает только известные id', () => {
    expect(isValidTheme('dark')).toBe(true)
    expect(isValidTheme('neon-hacker')).toBe(false)
    expect(isValidTheme('')).toBe(false)
    expect(isValidTheme(null)).toBe(false)
    expect(isValidTheme(undefined)).toBe(false)
    expect(isValidTheme(42)).toBe(false)
  })

  it('findTheme отдаёт опцию по id и null для неизвестной', () => {
    expect(findTheme('dark')?.name).toBe('Тёмная')
    expect(findTheme('neon-hacker')).toBeNull()
  })

  it('applyTheme ставит data-theme на <html>', () => {
    expect(applyTheme('corporate-orange')).toBe('corporate-orange')
    expect(document.documentElement.getAttribute('data-theme')).toBe('corporate-orange')
  })

  it('applyTheme схлопывает неизвестную тему в светлую', () => {
    expect(applyTheme('neon-hacker')).toBe(DEFAULT_THEME)
    expect(document.documentElement.getAttribute('data-theme')).toBe('light')
  })

  it('readStoredTheme игнорирует мусор в хранилище', () => {
    expect(readStoredTheme()).toBeNull()

    storeTheme('dark')
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('dark')
    expect(readStoredTheme()).toBe('dark')

    localStorage.setItem(THEME_STORAGE_KEY, 'neon-hacker')
    expect(readStoredTheme()).toBeNull()
  })

  // Тема применяется до первого кадра bootstrap-скриптом в index.html, поэтому
  // список id там продублирован. Замок: дубль не должен разъехаться с реестром -
  // иначе новая тема при загрузке молча схлопнется в светлую и мигнёт.
  it('bootstrap-скрипт index.html знает те же темы, что реестр', () => {
    const html = readFileSync(resolve(__dirname, '../../../index.html'), 'utf8')

    const keyMatch = html.match(/localStorage\.getItem\('([^']+)'\)/)
    expect(keyMatch, 'bootstrap должен читать ключ темы из localStorage').not.toBeNull()
    expect(keyMatch[1]).toBe(THEME_STORAGE_KEY)

    const listMatch = html.match(/var known = \[([^\]]+)\]/)
    expect(listMatch, 'bootstrap должен содержать список известных тем').not.toBeNull()
    const bootstrapIds = listMatch[1].split(',').map((s) => s.trim().replace(/^'|'$/g, ''))
    expect(bootstrapIds).toEqual(THEMES.map((t) => t.id))

    // Дефолт стоит атрибутом на <html>: до выполнения скрипта тема уже есть.
    expect(html).toMatch(/<html[^>]*data-theme="light"/)
  })

  // tokens.css - вторая половина того же контракта: у каждой темы должен быть
  // свой блок переменных, иначе выбор темы даст пустую палитру.
  it('tokens.css задаёт палитру каждой темы реестра', () => {
    const css = readFileSync(resolve(__dirname, '../../assets/tokens.css'), 'utf8')

    THEMES.forEach((theme) => {
      expect(css, `нет блока :root[data-theme="${theme.id}"]`)
        .toContain(`:root[data-theme="${theme.id}"]`)
    })

    // Роли акцента: заливка, акцент-как-текст и подпись на заливке - разные
    // переменные, и каждая тема обязана задать все три.
    const blocks = css.split(/:root(?:,\s*:root)?\[data-theme="/).slice(1)
    expect(blocks).toHaveLength(THEMES.length)
    blocks.forEach((block) => {
      ['--accent:', '--accent-text:', '--accent-contrast:', '--text-muted:', '--unread-bg:']
        .forEach((v) => expect(block).toContain(v))
    })
  })

  // Смена темы должна ложиться одним кадром: у меню и таблицы свой transition на
  // цвет, и без гашения они доезжают вразнобой (#1415).
  it('гасит CSS-переходы на время смены и снимает класс после кадра', () => {
    const frames = []
    const rafSpy = vi.spyOn(window, 'requestAnimationFrame')
      .mockImplementation((cb) => { frames.push(cb); return frames.length })

    applyTheme('dark')

    expect(document.documentElement.classList.contains('theme-switching'),
      'на момент смены переходы должны быть выключены').toBe(true)
    frames.shift()()   // первый кадр - браузер применил новые цвета
    frames.shift()()   // второй - снимаем запрет
    expect(document.documentElement.classList.contains('theme-switching')).toBe(false)
    rafSpy.mockRestore()
  })

  it('снимает гашение по таймеру, если кадры не приходят (фоновая вкладка)', () => {
    vi.useFakeTimers()
    const rafSpy = vi.spyOn(window, 'requestAnimationFrame').mockImplementation(() => 1)

    applyTheme('dark')
    expect(document.documentElement.classList.contains('theme-switching')).toBe(true)
    vi.advanceTimersByTime(300)

    expect(document.documentElement.classList.contains('theme-switching'),
      'класс не должен зависать - иначе переходы останутся выключенными').toBe(false)
    rafSpy.mockRestore()
    vi.useRealTimers()
  })

  it('tokens.css выключает переходы под этим классом', () => {
    const tokens = readFileSync(resolve(__dirname, '../../assets/tokens.css'), 'utf8')
    expect(tokens).toMatch(/html\.theme-switching[\s\S]{0,200}transition:\s*none\s*!important/)
  })

  // Подпись на цветной заливке (#1415): в тёмных темах заливка тёмная, подпись
  // светлая - иначе получается «чёрный текст по красному/синему».
  it('каждая тема задаёт подпись на заливке, и в тёмных она проходит AA', () => {
    const tokens = readFileSync(resolve(__dirname, '../../assets/tokens.css'), 'utf8')
    const lum = (hex) => {
      const ch = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255)
        .map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4))
      return 0.2126 * ch[0] + 0.7152 * ch[1] + 0.0722 * ch[2]
    }
    const ratio = (a, b) => (Math.max(lum(a), lum(b)) + 0.05) / (Math.min(lum(a), lum(b)) + 0.05)
    const blockOf = (theme) => {
      const start = tokens.indexOf(`[data-theme="${theme}"] {`)
      return tokens.slice(start, tokens.indexOf('\n}', start))
    }
    const value = (block, name) => block.match(new RegExp(`${name}:\\s*(#[0-9a-fA-F]{6})`))?.[1]

    THEMES.forEach(({ id }) => {
      const block = blockOf(id)
      expect(value(block, '--fill-text'), `${id}: нет --fill-text`).toBeTruthy()
    })

    ;['dark', 'dark-orange'].forEach((id) => {
      const block = blockOf(id)
      const fill = value(block, '--fill-text')
      ;['--accent', '--danger', '--success', '--warning', '--info'].forEach((name) => {
        const bg = value(block, name)
        expect(ratio(fill, bg), `${id}: подпись на ${name} даёт слабый контраст`).toBeGreaterThan(4.5)
        // И текст роли на её подложке: чипы «Объявление»/«Предупреждение», теги.
        const soft = value(block, `${name}-bg`)
        const text = value(block, `${name}-text`)
        if (soft && text) {
          expect(ratio(text, soft), `${id}: ${name}-text на ${name}-bg слабый`).toBeGreaterThan(4.5)
        }
      })
    })
  })
})
