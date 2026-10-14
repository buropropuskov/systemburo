import fs from 'node:fs'
import path from 'node:path'
import zlib from 'node:zlib'

/** Статические JS/CSS зависимости. Dynamic imports считаются только для выбранного экрана. */
export function collectAssets(manifest, entries) {
  const visited = new Set()
  const assets = new Set()
  function visit(key) {
    if (visited.has(key)) return
    const item = manifest[key]
    if (!item || typeof item.file !== 'string') throw new Error(`Нет записи manifest: ${key}`)
    visited.add(key)
    if (/\.(?:js|css)$/.test(item.file)) assets.add(item.file)
    for (const css of item.css || []) assets.add(css)
    for (const dependency of item.imports || []) visit(dependency)
  }
  entries.forEach(visit)
  return [...assets].sort()
}

export function measureAssets(dist, assets) {
  const root = path.resolve(dist)
  return assets.map((file) => {
    const resolved = path.resolve(root, file)
    if (!resolved.startsWith(root + path.sep)) throw new Error(`Недопустимый путь ресурса: ${file}`)
    const raw = fs.readFileSync(resolved)
    return { file, rawBytes: raw.length, gzipBytes: zlib.gzipSync(raw, { level: 9 }).length }
  }).sort((a, b) => b.gzipBytes - a.gzipBytes)
}

export function checkLimits(metrics, limits) {
  const total = metrics.reduce((sum, asset) => sum + asset.gzipBytes, 0)
  const largest = Math.max(0, ...metrics.map((asset) => asset.gzipBytes))
  const problems = []
  for (const [label, actual, maximum] of [
    ['gzip bytes', total, limits.gzipKb * 1024],
    ['largest gzip bytes', largest, limits.largestChunkGzipKb * 1024],
    ['requests', metrics.length, limits.requests],
  ]) {
    if (maximum !== undefined && Number.isFinite(maximum) && actual > maximum) {
      problems.push(`${label}: ${actual} > ${maximum}`)
    }
  }
  return { total, largest, requests: metrics.length, problems }
}
