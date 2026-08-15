import fs from 'node:fs'
import path from 'node:path'

import { describe, it, expect } from 'vitest'

/*
 * Разрешительные лицензии (MIT, Apache-2.0, BSD, ISC) разрешают пользоваться
 * чужим кодом при одном условии - уведомление об авторских правах должно
 * сохраняться в поставке. Условие выполняет THIRD-PARTY-NOTICES.md в корне
 * репозитория, а он собирается сценарием scripts/gen-third-party-notices.py.
 *
 * Забыть его запустить ничего не стоит: новая библиотека сборку не ломает и
 * тесты не красит, поэтому расхождение дожило бы до заказчика. Замок сверяет
 * перечень с производственным замыканием зависимостей в обе стороны: добавленный
 * пакет обязан появиться в перечне, удалённый - исчезнуть из него.
 *
 * Замыкание берётся из файла замка, а не из установленных пакетов: тест не
 * должен требовать npm ci ради проверки текстового файла.
 */

const repoRoot = path.join(import.meta.dirname, '..', '..', '..')
const lock = JSON.parse(fs.readFileSync(path.join(repoRoot, 'frontend', 'package-lock.json'), 'utf8'))
const notices = fs.readFileSync(path.join(repoRoot, 'THIRD-PARTY-NOTICES.md'), 'utf8')

const REMEDY = 'выполните python3 scripts/gen-third-party-notices.py и закоммитьте результат'

/**
 * Строки таблицы раздела о компонентах интерфейса: имя пакета -> версии.
 * @returns {Map<string, Set<string>>}
 */
function listedComponents() {
  const start = notices.search(/^## \d+\. Компоненты интерфейса$/m)
  expect(start, 'в перечне нет раздела о компонентах интерфейса').toBeGreaterThan(-1)

  const tail = notices.slice(start + 1)
  const end = tail.search(/^## /m)
  const section = end === -1 ? tail : tail.slice(0, end)

  const found = new Map()
  for (const [, name, version] of section.matchAll(/^\| `([^`]+)` \| ([^|]+) \|/gm)) {
    if (!found.has(name)) found.set(name, new Set())
    found.get(name).add(version.trim())
  }
  return found
}

/**
 * Производственное замыкание из файла замка: всё, до чего дотягиваются
 * dependencies. Пакеты, собранные под чужую платформу, пропускаются - они
 * значатся в замке, но не устанавливаются и в поставку не попадают.
 * @returns {Map<string, Set<string>>}
 */
function productionClosure() {
  const found = new Map()
  Object.entries(lock.packages).forEach(([key, entry]) => {
    if (!key || entry.dev) return
    if (entry.os || entry.cpu) return
    const name = key.split('node_modules/').pop()
    if (!found.has(name)) found.set(name, new Set())
    found.get(name).add(entry.version)
  })
  return found
}

describe('перечень сторонних компонентов', () => {
  const listed = listedComponents()
  const closure = productionClosure()

  it('находит зависимости, по которым идёт проверка', () => {
    expect(closure.size).toBeGreaterThan(100)
    expect(listed.size).toBeGreaterThan(100)
  })

  it('называет каждую зависимость, уходящую в поставку', () => {
    const missing = []
    closure.forEach((versions, name) => {
      versions.forEach((version) => {
        if (!listed.get(name)?.has(version)) missing.push(`${name} ${version}`)
      })
    })
    expect(missing.sort(), `зависимости нет в перечне: ${REMEDY}`).toEqual([])
  })

  it('не называет того, чего в зависимостях уже нет', () => {
    const installed = new Map()
    Object.entries(lock.packages).forEach(([key, entry]) => {
      if (!key || entry.dev) return
      const name = key.split('node_modules/').pop()
      if (!installed.has(name)) installed.set(name, new Set())
      installed.get(name).add(entry.version)
    })

    const orphaned = []
    listed.forEach((versions, name) => {
      versions.forEach((version) => {
        if (!installed.get(name)?.has(version)) orphaned.push(`${name} ${version}`)
      })
    })
    expect(orphaned.sort(), `перечень отстал от зависимостей: ${REMEDY}`).toEqual([])
  })

  it('приводит текст лицензии, а не одно её название', () => {
    const blocks = notices.match(/^```text$/gm) ?? []
    expect(blocks.length).toBeGreaterThan(100)
  })

  it('выносит отдельно то, что разрешительной лицензией не покрыто', () => {
    // apexcharts с четвёртой версии распространяется по двойной лицензии:
    // бесплатная годится организациям с выручкой ниже порога, который заказчик
    // перешагивает. Пока пакет в зависимостях, перечень обязан называть это
    // прямо, а не растворять его среди двух сотен строк MIT.
    const apex = /^\| `apexcharts` \|/m.test(notices)
    if (!apex) return
    const section = notices.search(/^## \d+\. Условия, требующие отдельного внимания$/m)
    expect(section, 'apexcharts в перечне есть, а раздела об особых условиях нет').toBeGreaterThan(-1)
  })
})
