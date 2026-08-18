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
 * Текст раздела по его заголовку, до следующего заголовка того же уровня.
 * @param {RegExp} heading заголовок раздела
 * @returns {string}
 */
function section(heading) {
  const start = notices.search(heading)
  expect(start, `в перечне нет раздела ${heading}`).toBeGreaterThan(-1)
  const tail = notices.slice(start + 1)
  const end = tail.search(/^## /m)
  return end === -1 ? tail : tail.slice(0, end)
}

/**
 * Строки таблицы раздела: имя пакета -> версии.
 * @param {string} text текст раздела
 * @returns {Map<string, Set<string>>}
 */
function rowsOf(text) {
  const found = new Map()
  for (const [, name, version] of text.matchAll(/^\| `([^`]+)` \| ([^|]+) \|/gm)) {
    if (!found.has(name)) found.set(name, new Set())
    found.get(name).add(version.trim())
  }
  return found
}

/**
 * Производственное замыкание из файла замка.
 *
 * Сборки под конкретную платформу (у них в замке заданы os или cpu) идут
 * отдельным списком: в замке значатся все сразу, а устанавливается одна, и
 * перечень собирает их из замка, не заглядывая на диск.
 * @returns {{main: Map<string, Set<string>>, platform: Map<string, Set<string>>}}
 */
function productionClosure() {
  const main = new Map()
  const platform = new Map()
  Object.entries(lock.packages).forEach(([key, entry]) => {
    if (!key || entry.dev) return
    const target = entry.os || entry.cpu ? platform : main
    const name = key.split('node_modules/').pop()
    if (!target.has(name)) target.set(name, new Set())
    target.get(name).add(entry.version)
  })
  return { main, platform }
}

/**
 * Пары «имя версия», которые есть в ожидаемом наборе и отсутствуют в перечне.
 * @param {Map<string, Set<string>>} expected
 * @param {Map<string, Set<string>>} listed
 * @returns {string[]}
 */
function absent(expected, listed) {
  const gaps = []
  expected.forEach((versions, name) => {
    versions.forEach((version) => {
      if (!listed.get(name)?.has(version)) gaps.push(`${name} ${version}`)
    })
  })
  return gaps.sort()
}

describe('перечень сторонних компонентов', () => {
  const interfaceRows = rowsOf(section(/^## \d+\. Компоненты интерфейса$/m))
  const platformRows = rowsOf(section(/^## \d+\. Сборки под конкретную платформу$/m))
  const closure = productionClosure()

  it('находит зависимости, по которым идёт проверка', () => {
    expect(closure.main.size).toBeGreaterThan(100)
    expect(interfaceRows.size).toBeGreaterThan(100)
    expect(closure.platform.size).toBeGreaterThan(0)
  })

  it('называет каждую зависимость, уходящую в поставку', () => {
    expect(absent(closure.main, interfaceRows), `зависимости нет в перечне: ${REMEDY}`).toEqual([])
  })

  // Сборки под чужую платформу в поставку не попадают, но перечень их называет:
  // иначе он зависел бы от того, на какой машине его собрали, и на macOS
  // получался бы другой файл, чем в контейнере.
  it('называет сборки под каждую платформу, а не под свою', () => {
    expect(absent(closure.platform, platformRows), `сборки нет в перечне: ${REMEDY}`).toEqual([])
  })

  it('не называет того, чего в зависимостях уже нет', () => {
    const installed = new Map()
    closure.main.forEach((versions, name) => installed.set(name, new Set(versions)))
    closure.platform.forEach((versions, name) => {
      if (!installed.has(name)) installed.set(name, new Set())
      versions.forEach((version) => installed.get(name).add(version))
    })

    const listed = new Map(interfaceRows)
    platformRows.forEach((versions, name) => {
      if (!listed.has(name)) listed.set(name, new Set())
      versions.forEach((version) => listed.get(name).add(version))
    })

    expect(absent(listed, installed), `перечень отстал от зависимостей: ${REMEDY}`).toEqual([])
  })

  it('приводит текст лицензии, а не одно её название', () => {
    const blocks = notices.match(/^```text$/gm) ?? []
    expect(blocks.length).toBeGreaterThan(100)
  })

  // apexcharts с четвёртой версии распространяется по двойной лицензии:
  // бесплатная годится организациям с выручкой ниже порога, который заказчик
  // перешагивает. Графики переведены на chart.js (MIT), а пакет убран из
  // зависимостей - вернуть его значит вернуть и обязательство, поэтому дверь
  // держим закрытой замком, а не памятью. Понадобится обратно - решение
  // принимает человек, и тогда правится этот тест вместе с перечнем.
  it('не тянет apexcharts обратно: движок графиков ушёл на chart.js', () => {
    const returned = [...closure.main.keys(), ...closure.platform.keys()]
      .filter((name) => name.includes('apexcharts'))
    expect(returned, 'apexcharts снова в зависимостях - это лицензионное решение').toEqual([])
  })
})
