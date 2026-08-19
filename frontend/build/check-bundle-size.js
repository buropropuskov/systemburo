#!/usr/bin/env node
/*
 * Бюджет стартовой загрузки.
 *
 * Считаем ровно то, что браузер тянет до первого экрана: модуль-точку входа,
 * все modulepreload и стили из dist/index.html. Ленивые чанки роутов сюда не
 * попадают, поэтому число отражает цену открытия страницы логина, а не размер
 * сборки целиком.
 *
 * Меряем в gzip: по сети едет он, и разница существенная - ExcelJS весит
 * 908 КБ сырыми и 249 КБ сжатыми. Уровень сжатия фиксирован, чтобы число не
 * плавало между машинами.
 *
 * Бюджет держится чуть выше текущего размера и опускается по мере работы над
 * бандлом. Вырос - CI краснеет и показывает, какой файл потяжелел.
 *
 * Запуск: npm run build && node build/check-bundle-size.js [--update]
 */
import fs from 'node:fs'
import path from 'node:path'
import zlib from 'node:zlib'
import { fileURLToPath } from 'node:url'

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const DIST = path.join(ROOT, 'dist')
const INDEX = path.join(DIST, 'index.html')
const BUDGET_PATH = path.join(ROOT, 'build', 'bundle-budget.json')

if (!fs.existsSync(INDEX)) {
  console.error('Нет dist/index.html. Сначала "npm run build".')
  process.exit(1)
}

const budget = JSON.parse(fs.readFileSync(BUDGET_PATH, 'utf8'))
const KB = 1024

/** Ассеты, которые браузер грузит до первого экрана: точка входа, modulepreload, стили. */
function initialAssets() {
  const html = fs.readFileSync(INDEX, 'utf8')
  return [...new Set([...html.matchAll(/(?:src|href)="(\/assets\/[^"]+)"/g)].map((m) => m[1]))]
}

function gzipSize(relUrl) {
  const file = path.join(DIST, relUrl.replace(/^\//, ''))
  return zlib.gzipSync(fs.readFileSync(file), { level: 9 }).length
}

const assets = initialAssets()
  .map((url) => ({ url, gzip: gzipSize(url) }))
  .sort((a, b) => b.gzip - a.gzip)

const totalGzip = assets.reduce((sum, a) => sum + a.gzip, 0)
const largest = assets[0]

if (process.argv.includes('--update')) {
  const next = {
    ...budget,
    initialGzipKb: Math.ceil((totalGzip / KB) * 1.02),
    largestChunkGzipKb: Math.ceil((largest.gzip / KB) * 1.02),
  }
  fs.writeFileSync(BUDGET_PATH, `${JSON.stringify(next, null, 2)}\n`)
  console.log(`Бюджет обновлён: стартовая загрузка ${next.initialGzipKb} КБ, крупнейший чанк ${next.largestChunkGzipKb} КБ.`)
  process.exit(0)
}

console.log('Стартовая загрузка (gzip):')
for (const a of assets.slice(0, 8)) {
  console.log(`  ${(a.gzip / KB).toFixed(0).padStart(5)} КБ  ${a.url}`)
}
if (assets.length > 8) console.log(`  ... и ещё ${assets.length - 8} файлов`)

const totalKb = totalGzip / KB
const largestKb = largest.gzip / KB
const problems = []
if (totalKb > budget.initialGzipKb) {
  problems.push(`стартовая загрузка ${totalKb.toFixed(0)} КБ при бюджете ${budget.initialGzipKb} КБ`)
}
if (largestKb > budget.largestChunkGzipKb) {
  problems.push(`крупнейший чанк ${largest.url} ${largestKb.toFixed(0)} КБ при бюджете ${budget.largestChunkGzipKb} КБ`)
}

console.log(`Итого: ${totalKb.toFixed(0)} КБ gzip в ${assets.length} файлах, бюджет ${budget.initialGzipKb} КБ.`)

if (problems.length > 0) {
  console.error('')
  console.error('Бюджет бандла превышен:')
  for (const p of problems) console.error(`  ${p}`)
  console.error('')
  console.error('Либо унеси тяжёлое в динамический import(), либо, если рост осознанный,')
  console.error('подними бюджет через "npm run lint:bundle -- --update" и объясни в описании PR.')
  process.exit(1)
}
