#!/usr/bin/env node
/*
 * Гейт на размер файлов - храповик, а не разовая проверка.
 *
 * Новый файл обязан уложиться в порог. Файл, который уже сверх порога, не может
 * вырасти дальше: дописал сотню строк в компонент на четыре тысячи - CI краснеет.
 * Ужимать можно всегда.
 *
 * Сравниваем с общим предком текущей ветки и dev, а не с зафиксированным
 * снимком размеров. Снимок на активной ветке гоняется за своим хвостом: пока
 * идёт ревью, кто-то дописывает строки в другом PR, и проверка краснеет на
 * правке, которой автор не делал. С merge-base каждый отвечает ровно за то,
 * что изменил сам, и файла с базой держать не нужно.
 *
 * Пороги заданы на блок, а не на файл: у .vue три блока с разной природой, и
 * тысяча строк стилей рядом с двумя сотнями логики - это не то же самое, что
 * тысяча строк логики. По мере распила мега-компонентов пороги опускаются.
 *
 * Запуск: node build/check-file-sizes.js [--base <ref>]
 */
import fs from 'node:fs'
import path from 'node:path'
import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const SCAN_DIR = path.join(ROOT, 'src')
const REPO = path.resolve(ROOT, '..')

const TARGETS = { template: 400, script: 500, style: 500, js: 400 }

/**
 * Вызов git. При allowFail возвращает null вместо исключения и не пускает stderr
 * наружу: отсутствие файла в базовом коммите - штатная ветка (файл новый), а
 * "fatal: path ... exists on disk, but not in ..." в выводе читается как поломка.
 */
function git(args, allowFail = false) {
  try {
    return execFileSync('git', args, {
      cwd: REPO,
      encoding: 'utf8',
      maxBuffer: 64 * 1024 * 1024,
      stdio: allowFail ? ['ignore', 'pipe', 'ignore'] : ['ignore', 'pipe', 'inherit'],
    })
  } catch (err) {
    if (allowFail) return null
    throw err
  }
}

/**
 * Коммит, с которым сравниваем: общий предок HEAD и целевой ветки.
 *
 * В проверке пулл-реквеста GitHub собирает merge-коммит, поэтому целевая ветка
 * уже влита в HEAD и общий предок совпадает с её вершиной - сравнение идёт с
 * актуальным dev. Локально это точка, от которой отведена ветка.
 */
function mergeBase() {
  const flagIndex = process.argv.indexOf('--base')
  const explicit = flagIndex !== -1 ? process.argv[flagIndex + 1] : null
  const candidates = explicit
    ? [explicit]
    : [`origin/${process.env.GITHUB_BASE_REF || 'dev'}`, 'origin/dev', 'dev']

  for (const ref of candidates) {
    const resolved = git(['rev-parse', '--verify', '--quiet', `${ref}^{commit}`], true)
    if (!resolved) continue
    const base = git(['merge-base', 'HEAD', resolved.trim()], true)
    if (base) return { ref, sha: base.trim() }
  }
  return null
}

/** Файл относится к тестам - на них пороги не распространяются. */
function isTest(filePath) {
  return filePath.includes(`${path.sep}__tests__${path.sep}`) || filePath.endsWith('.spec.js')
}

function walk(dir, acc = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) walk(full, acc)
    else if (/\.(vue|js)$/.test(entry.name) && !isTest(full)) acc.push(full)
  }
  return acc
}

/**
 * Строки внутри блока SFC, без самих тегов. Несколько блоков одного типа
 * (script setup рядом с script, два style) суммируются.
 *
 * Теги корневых блоков ищем только с нулевым отступом. Иначе вложенный
 * `<template v-if>` внутри шаблона закрывает блок на своём `</template>`,
 * и весь остаток файла в счёт не идёт.
 *
 * @param {string} source содержимое .vue
 * @param {string} tag template | script | style
 * @returns {number}
 */
function blockLines(source, tag) {
  const open = new RegExp(`^<${tag}(?:\\s[^>]*)?>\\s*$`)
  const close = new RegExp(`^</${tag}>\\s*$`)
  let total = 0
  let openLine = -1
  source.split('\n').forEach((line, i) => {
    if (openLine === -1) {
      if (open.test(line)) openLine = i
    } else if (close.test(line)) {
      total += i - openLine - 1
      openLine = -1
    }
  })
  return total
}

/**
 * Размеры по измерениям, к которым применимы пороги.
 *
 * @param {string} source содержимое файла
 * @param {boolean} isVue
 * @returns {Record<string, number>}
 */
function measure(source, isVue) {
  if (!isVue) return { js: source.split('\n').length }
  return {
    template: blockLines(source, 'template'),
    script: blockLines(source, 'script'),
    style: blockLines(source, 'style'),
  }
}

const base = mergeBase()
if (!base) {
  console.error('Не удалось определить базовый коммит для сравнения.')
  console.error('Нужна история целевой ветки: в CI это fetch-depth: 0 у actions/checkout,')
  console.error('локально - "git fetch origin dev". Явно задать: --base <ref>.')
  process.exit(1)
}

const failures = []

for (const file of walk(SCAN_DIR)) {
  const rel = path.relative(REPO, file).split(path.sep).join('/')
  const isVue = file.endsWith('.vue')
  const current = measure(fs.readFileSync(file, 'utf8'), isVue)

  const over = Object.entries(current).filter(([dim, value]) => value > TARGETS[dim])
  if (over.length === 0) continue

  const previousSource = git(['show', `${base.sha}:${rel}`], true)
  const previous = previousSource === null ? null : measure(previousSource, isVue)

  for (const [dim, value] of over) {
    if (previous === null) {
      failures.push(`${rel} [${dim}]: ${value} строк, порог ${TARGETS[dim]}. Новый файл обязан уложиться в порог - вынеси часть в отдельный модуль.`)
    } else if (value > previous[dim]) {
      failures.push(`${rel} [${dim}]: было ${previous[dim]}, стало ${value}. Файл и так сверх порога ${TARGETS[dim]}, расти ему нельзя.`)
    }
  }
}

if (failures.length > 0) {
  console.error(`Размер файлов: ${failures.length} нарушений (сравнение с ${base.ref}, ${base.sha.slice(0, 8)}).`)
  for (const line of failures) console.error(`  ${line}`)
  console.error('')
  console.error(`Пороги на блок: ${Object.entries(TARGETS).map(([k, v]) => `${k} ${v}`).join(', ')}.`)
  process.exit(1)
}

console.log(`Размер файлов: порядок. Сравнение с ${base.ref} (${base.sha.slice(0, 8)}), выросших сверх порога нет.`)
