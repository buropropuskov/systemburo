// @vitest-environment node
import { describe, it, expect } from 'vitest'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { collectAssets, measureAssets, checkLimits } from '../../build/bundle-metrics.js'

const manifest = {
  'index.html': { file: 'assets/index.js', imports: ['shell'], dynamicImports: ['page'] },
  shell: { file: 'assets/shell.js', css: ['assets/shared.css'], imports: ['runtime'] },
  runtime: { file: 'assets/runtime.js', imports: ['shell'] },
  page: { file: 'assets/page.js', css: ['assets/shared.css', 'assets/page.css'], imports: ['runtime'] },
}

describe('расчёт статических ресурсов экрана', () => {
  it('учитывает транзитивные импорты и CSS, останавливает цикл, исключает dynamic imports', () => {
    expect(collectAssets(manifest, ['index.html'])).toEqual([
      'assets/index.js', 'assets/runtime.js', 'assets/shared.css', 'assets/shell.js',
    ])
  })
  it('считает первый экран вместе с оболочкой без повторов', () => {
    expect(collectAssets(manifest, ['index.html', 'page'])).toEqual([
      'assets/index.js', 'assets/page.css', 'assets/page.js', 'assets/runtime.js',
      'assets/shared.css', 'assets/shell.js',
    ])
  })
  it('не пропускает неизвестный экран или отсутствующую зависимость', () => {
    expect(() => collectAssets(manifest, ['missing'])).toThrow('Нет записи manifest')
    expect(() => collectAssets({ a: { file: 'a.js', imports: ['missing'] } }, ['a'])).toThrow('missing')
  })
  it('сравнивает точные байты, включая превышение на один байт', () => {
    expect(checkLimits([{ gzipBytes: 1024 }], { gzipKb: 1, requests: 1 }).problems).toEqual([])
    expect(checkLimits([{ gzipBytes: 1025 }], { gzipKb: 1 }).problems).toEqual(['gzip bytes: 1025 > 1024'])
    expect(checkLimits([{ gzipBytes: 600 }, { gzipBytes: 500 }], {
      gzipKb: 2, requests: 1, largestChunkGzipKb: 0.5,
    }).problems).toEqual(['largest gzip bytes: 600 > 512', 'requests: 2 > 1'])
  })
  it('измеряет реальные файлы и не допускает выход за каталог сборки', () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'bundle-metrics-'))
    try {
      fs.writeFileSync(path.join(dir, 'entry.js'), 'export const value = 42;')
      const result = measureAssets(dir, ['entry.js'])
      expect(result[0]).toMatchObject({ file: 'entry.js', rawBytes: 24 })
      expect(result[0].gzipBytes).toBeGreaterThan(0)
      expect(measureAssets(dir, ['entry.js'])).toEqual(result)
      expect(() => measureAssets(dir, ['../entry.js'])).toThrow('Недопустимый путь')
      expect(() => measureAssets(dir, ['missing.js'])).toThrow()
    } finally {
      fs.rmSync(dir, { recursive: true, force: true })
    }
  })
})
