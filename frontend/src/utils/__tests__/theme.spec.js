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

  it('реестр содержит светлую и тёмную тему', () => {
    const ids = THEMES.map((t) => t.id)
    expect(ids).toEqual(['light', 'dark'])
    expect(new Set(ids).size).toBe(ids.length)
    THEMES.forEach((t) => {
      expect(t.name).toBeTruthy()
      expect(t.dot).toMatch(/^#[0-9a-fA-F]{6}$/)
    })
  })

  it('isValidTheme пропускает только известные id', () => {
    expect(isValidTheme('dark')).toBe(true)
    // Снятые темы (#1415): в профиле у кого-то могло остаться старое значение -
    // оно обязано схлопнуться в светлую, а не оставить пустую палитру.
    expect(isValidTheme('corporate-orange')).toBe(false)
    expect(applyTheme('corporate-orange')).toBe(DEFAULT_THEME)
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
    expect(applyTheme('dark')).toBe('dark')
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
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

  // Нативные контролы (индикатор календаря у input[type=date], скроллбары, попап
  // select) рисует браузер, и наши переменные ему не видны - их перекрашивает
  // только color-scheme. Без него в тёмной теме иконка календаря оставалась
  // чёрной на тёмном поле.
  it('каждая тема объявляет color-scheme, тёмные - dark', () => {
    const css = readFileSync(resolve(__dirname, '../../assets/tokens.css'), 'utf8')
    const blocks = css.split(/:root(?:,\s*:root)?\[data-theme="/).slice(1)

    blocks.forEach((block) => {
      const id = block.slice(0, block.indexOf('"'))
      const scheme = block.match(/color-scheme:\s*(\w+);/)
      expect(scheme, `тема ${id} не объявила color-scheme`).not.toBeNull()
      expect(scheme[1], `тема ${id}`).toBe(id.startsWith('dark') ? 'dark' : 'light')
    })
  })

  // Подложки строк «непрочитано» и «обновлено» обязаны нести ЦВЕТ роли: нейтральные
  // серые отличались от карточки на 5% яркости, и на телефоне (там от подсветки
  // остаётся полоса слева) роли было не различить.
  it('в тёмных темах подложки строк тонированы и различимы между собой', () => {
    const css = readFileSync(resolve(__dirname, '../../assets/tokens.css'), 'utf8')
    const rgb = (hex) => [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16))
    // Отрыв от поверхности считаем по всем каналам, а не по светлоте: глаз ловит
    // именно тон, и нейтральный серый на 5% светлее карточки визуально не читался.
    const shift = (a, b) => a.reduce((s, v, i) => s + Math.abs(v - b[i]), 0)

    css.split(/:root(?:,\s*:root)?\[data-theme="/).slice(1)
      .filter((block) => block.startsWith('dark'))
      .forEach((block) => {
        const id = block.slice(0, block.indexOf('"'))
        const val = (name) => rgb(block.match(new RegExp(`${name}:\\s*(#[0-9a-fA-F]{6});`))[1])
        const [unread, updated, surface] = ['--unread-bg', '--updated-bg', '--surface'].map(val)

        // Тёплая роль краснее синего канала, лиловая - наоборот.
        expect(unread[0], `${id}: «непрочитано» должно быть тёплым`).toBeGreaterThan(unread[2])
        expect(updated[2], `${id}: «обновлено» должно быть лиловым`).toBeGreaterThan(updated[0])
        ;[['--unread-bg', unread], ['--updated-bg', updated]].forEach(([name, c]) => {
          expect(shift(c, surface), `${id}: ${name} почти не отличается от --surface`)
            .toBeGreaterThan(30)
        })
        // И роли не путаются между собой.
        expect(shift(unread, updated), `${id}: роли строк слишком похожи`).toBeGreaterThan(30)
      })
  })

  // Карточка-подсказка в тёмных темах - утопленная панель в тон темы, не акцентная
  // заливка: сплошной синий блок на тёмном фоне читался плохо.
  it('в тёмных темах подсказка темнее поверхности карточки', () => {
    const css = readFileSync(resolve(__dirname, '../../assets/tokens.css'), 'utf8')
    const lum = (hex) => {
      const [r, g, b] = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255)
      return 0.2126 * r + 0.7152 * g + 0.0722 * b
    }
    css.split(/:root(?:,\s*:root)?\[data-theme="/).slice(1)
      .filter((block) => block.startsWith('dark'))
      .forEach((block) => {
        const id = block.slice(0, block.indexOf('"'))
        const hint = block.match(/--hint-card-bg:\s*(#[0-9a-fA-F]{6});/)
        const surface = block.match(/--surface:\s*(#[0-9a-fA-F]{6});/)
        expect(hint, `${id}: подсказка должна быть своим цветом, не var(--accent)`).not.toBeNull()
        expect(lum(hint[1]), `${id}: ${hint[1]} не темнее поверхности ${surface[1]}`)
          .toBeLessThan(lum(surface[1]))
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

    ;['dark'].forEach((id) => {
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

/*
 * Контракт оформления, который легко потерять при правках CSS: нативная отрисовка
 * кнопок и рамки тегов списков (#1415).
 */
describe('оформление: кнопки и теги', () => {
  const read = (rel) => readFileSync(resolve(__dirname, rel), 'utf8')

  it('кнопки не полагаются на нативную отрисовку', () => {
    // Под color-scheme: dark браузер рисует <button> тёмным ButtonFace с рельефной
    // рамкой - кнопка без своего фона выглядит выпуклой.
    const css = read('../../assets/tokens.css')
    const rule = css.match(/(^|\n)button\s*\{([^}]*)\}/)
    expect(rule, 'нет правила button в tokens.css').not.toBeNull()
    expect(rule[2]).toMatch(/appearance:\s*none/)
  })

  it('пелена загрузки не красится токеном затемнения', () => {
    // --overlay это подложка ПОД модалкой (полупрозрачный чёрный). Для пелены
    // поверх таблицы он даёт затемнение всего списка при поиске и обновлении.
    const css = read('../../views/ApplicationsCenter.vue')
    const rule = css.match(/\.refresh-overlay\s*\{([^}]*)\}/)
    expect(rule, 'нет правила .refresh-overlay').not.toBeNull()
    expect(rule[1], 'пелена должна идти от --surface').not.toMatch(/background:[^;]*var\(--overlay\)/)
    expect(rule[1]).toMatch(/background:[^;]*var\(--surface\)/)
  })

  it('рамка тега цвета текста включается только в тёмной теме', () => {
    // В тёмной приглушённая color-mix-рамка Badge сливалась с подложкой, в светлой
    // она к месту - там вид остаётся прежним.
    //
    // Файл один: с #2319 теги рисует ApplicationTag и в Центре, и в кабинете. Копия
    // правила в UserApplications.vue после переезда стала мёртвой - scoped-стиль
    // родителя достаёт до КОРНЯ дочернего компонента (.application-tags), но не до
    // .rt-tag внутри него.
    for (const file of ['../../components/ApplicationTag.vue']) {
      const css = read(file)
      const darkRule = css.match(/\[data-theme="dark"\]\s+\.rt-tag\s*\{([^}]*)\}/)
      expect(darkRule, `${file}: нет правила рамки для тёмной темы`).not.toBeNull()
      expect(darkRule[1]).toMatch(/border-color:\s*currentColor/)

      const baseRule = css.match(/(?<!\]\s)\n\.rt-tag\s*\{([^}]*)\}/)
      if (baseRule) {
        expect(baseRule[1], `${file}: в светлой теме рамка должна остаться от Badge`)
          .not.toMatch(/border-color:\s*currentColor/)
      }
    }
  })
})

/*
 * Страницы-заглушки (500, обслуживание) рисуют собственный фон с декоративными
 * пятнами. Литералы держали их светлыми при любой теме - в тёмной получался
 * светло-серый текст на почти белом фоне (#1415).
 */
describe('страницы-заглушки следуют теме', () => {
  const pages = ['../../views/Error500.vue', '../../views/Maintenance.vue']

  it('фон страницы и пятна берутся от токенов, а не от литералов', () => {
    for (const rel of pages) {
      const css = readFileSync(resolve(__dirname, rel), 'utf8')
      const bg = css.match(/background:\s*\n?\s*radial-gradient[\s\S]*?;/)
      expect(bg, `${rel}: не найден фон страницы`).not.toBeNull()
      expect(bg[0], `${rel}: фон должен идти от --bg`).toContain('var(--bg)')
      expect(bg[0], `${rel}: в фоне остался литерал цвета`).not.toMatch(/#[0-9a-fA-F]{3,6}/)
    }
  })

  it('декоративная сетка тонируется переменной темы', () => {
    for (const rel of pages) {
      const css = readFileSync(resolve(__dirname, rel), 'utf8')
      const grid = css.match(/background-image:\s*\n?\s*linear-gradient[\s\S]*?;/)
      expect(grid, `${rel}: не найдена сетка`).not.toBeNull()
      expect(grid[0], `${rel}: сетка прошита синим литералом`).not.toMatch(/rgba?\(\s*79/)
      // Цвет линий приходит из --decor-line: в светлой теме это примесь акцента,
      // в тёмной - нейтральная рамка (акцентная сетка красила весь экран синим).
      expect(grid[0]).toMatch(/var\(--decor-line\)|var\(--accent\)/)
    }
  })
})
