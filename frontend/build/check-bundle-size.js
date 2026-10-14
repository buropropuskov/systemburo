#!/usr/bin/env node
// Расчётный gzip JS/CSS: shell и shell + выбранный экран, включая static imports.
// Это воспроизводимая метрика сборки, а не измерение реального сетевого сжатия.
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { collectAssets, measureAssets, checkLimits } from './bundle-metrics.js'

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const DIST = path.join(ROOT, 'dist')
const BUDGET_PATH = path.join(ROOT, 'build', 'bundle-budget.json')

try {
  const manifest = JSON.parse(fs.readFileSync(path.join(DIST, '.vite', 'manifest.json'), 'utf8'))
  const budget = JSON.parse(fs.readFileSync(BUDGET_PATH, 'utf8'))
  if (!budget.routes || !Object.keys(budget.routes).length) throw new Error('Не заданы проверяемые экраны')
  const screens = { shell: 'index.html', ...budget.routes }
  const results = Object.entries(screens).map(([name, config]) => {
    const entry = typeof config === 'string' ? config : config.entry
    const metrics = measureAssets(DIST, collectAssets(manifest, ['index.html', entry]))
    const limits = {
      gzipKb: name === 'shell' ? budget.initialGzipKb : config.gzipKb,
      requests: name === 'shell' ? budget.initialRequests : config.requests,
      largestChunkGzipKb: budget.largestChunkGzipKb,
    }
    if (!(limits.gzipKb > 0) || !(limits.requests > 0)) throw new Error(`Не заданы лимиты: ${name}`)
    return { name, metrics, limits, ...checkLimits(metrics, limits) }
  })

  if (process.argv.includes('--update')) {
    // Обновление — только явное ужесточение существующих лимитов; рост требует ручного решения.
    const shell = results[0]
    const next = structuredClone(budget)
    next.initialGzipKb = Math.min(budget.initialGzipKb, Math.ceil(shell.total / 1024 * 1.02))
    next.initialRequests = Math.min(budget.initialRequests, shell.requests)
    next.largestChunkGzipKb = Math.min(budget.largestChunkGzipKb,
      Math.ceil(Math.max(...results.map((result) => result.largest)) / 1024 * 1.02))
    for (const result of results.slice(1)) {
      next.routes[result.name].gzipKb = Math.min(budget.routes[result.name].gzipKb,
        Math.ceil(result.total / 1024 * 1.02))
      next.routes[result.name].requests = Math.min(budget.routes[result.name].requests, result.requests)
    }
    fs.writeFileSync(BUDGET_PATH, `${JSON.stringify(next, null, 2)}\n`)
    console.log('Существующие лимиты ужесточены по измеренной сборке; повышения не выполнялись.')
  }

  const problems = []
  console.log('Расчётный gzip JS/CSS, level 9; каждый экран включает общую оболочку:')
  for (const result of results) {
    console.log(`${result.name}: ${result.total} bytes (${(result.total / 1024).toFixed(2)} KiB), ` +
      `${result.requests} ресурсов; запас ${result.limits.gzipKb * 1024 - result.total} bytes`)
    for (const problem of result.problems) problems.push(`${result.name}: ${problem}`)
  }
  if (problems.length) throw new Error(`Бюджет превышен:\n${problems.join('\n')}`)
} catch (error) {
  console.error(error.message)
  console.error('Нужна production-сборка с manifest. Проверь состав ресурсов и утверждённые лимиты.')
  process.exitCode = 1
}
