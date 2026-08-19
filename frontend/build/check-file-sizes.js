#!/usr/bin/env node
/*
 * Гейт на размер файлов - храповик, а не разовая проверка.
 *
 * Новый файл обязан уложиться в порог. Существующие нарушители перечислены в
 * file-size-baseline.json со своим текущим размером и расти дальше не могут:
 * дописал сто строк в компонент на четыре тысячи - CI краснеет. Ужать файл
 * можно всегда, база при этом просто устаревает в мягкую сторону.
 *
 * Пороги заданы на блок, а не на файл: у .vue три блока с разной природой, и
 * тысяча строк стилей рядом с двумя сотнями логики - это не то же самое, что
 * тысяча строк логики. По мере распила мега-компонентов пороги опускаются, и
 * база пересобирается через `npm run lint:size -- --update`.
 *
 * Запуск: node build/check-file-sizes.js [--update]
 */
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const BASELINE_PATH = path.join(ROOT, 'build', 'file-size-baseline.json')
const SCAN_DIR = path.join(ROOT, 'src')

const baseline = JSON.parse(fs.readFileSync(BASELINE_PATH, 'utf8'))
const TARGETS = baseline.targets

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
 * Измеренные размеры файла по измерениям, к которым применимы пороги.
 *
 * @param {string} filePath
 * @returns {Record<string, number>}
 */
function measure(filePath) {
  const source = fs.readFileSync(filePath, 'utf8')
  if (filePath.endsWith('.vue')) {
    return {
      template: blockLines(source, 'template'),
      script: blockLines(source, 'script'),
      style: blockLines(source, 'style'),
    }
  }
  return { js: source.split('\n').length }
}

const measured = new Map()
for (const file of walk(SCAN_DIR)) {
  const rel = path.relative(ROOT, file).split(path.sep).join('/')
  const sizes = measure(file)
  const over = {}
  for (const [dim, value] of Object.entries(sizes)) {
    if (value > TARGETS[dim]) over[dim] = value
  }
  if (Object.keys(over).length > 0) measured.set(rel, over)
}

if (process.argv.includes('--update')) {
  const files = Object.fromEntries([...measured].sort(([a], [b]) => a.localeCompare(b)))
  fs.writeFileSync(BASELINE_PATH, `${JSON.stringify({ ...baseline, files }, null, 2)}\n`)
  console.log(`База обновлена: ${measured.size} файлов сверх порога.`)
  process.exit(0)
}

const failures = []
const shrunk = []

for (const [file, over] of measured) {
  const known = baseline.files[file]
  for (const [dim, value] of Object.entries(over)) {
    if (!known || known[dim] === undefined) {
      failures.push(`${file} [${dim}]: ${value} строк, порог ${TARGETS[dim]}. Файла нет в базе - уложись в порог или вынеси часть в отдельный модуль.`)
    } else if (value > known[dim]) {
      failures.push(`${file} [${dim}]: было ${known[dim]}, стало ${value}. Файл и так сверх порога ${TARGETS[dim]}, расти ему нельзя.`)
    } else if (value < known[dim]) {
      shrunk.push(`${file} [${dim}]: ${known[dim]} -> ${value}`)
    }
  }
}

for (const [file, dims] of Object.entries(baseline.files)) {
  const over = measured.get(file)
  for (const dim of Object.keys(dims)) {
    if (!over || over[dim] === undefined) shrunk.push(`${file} [${dim}]: уложился в порог`)
  }
}

if (shrunk.length > 0) {
  console.log(`Стало меньше (${shrunk.length}), база устарела в мягкую сторону - обнови через "npm run lint:size -- --update":`)
  for (const line of shrunk.slice(0, 10)) console.log(`  ${line}`)
  if (shrunk.length > 10) console.log(`  ... и ещё ${shrunk.length - 10}`)
  console.log('')
}

if (failures.length > 0) {
  console.error(`Размер файлов: ${failures.length} нарушений.`)
  for (const line of failures) console.error(`  ${line}`)
  console.error('')
  console.error(`Пороги на блок: ${Object.entries(TARGETS).map(([k, v]) => `${k} ${v}`).join(', ')}.`)
  process.exit(1)
}

console.log(`Размер файлов: порядок. Сверх порога ${measured.size} файлов, все зафиксированы базой.`)
