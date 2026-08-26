import fs from 'node:fs'
import path from 'node:path'

import { describe, it, expect } from 'vitest'

/*
 * Инлайновые глифы, скопированные из чужих наборов, вернулись бы в разметку
 * незаметно: путь в атрибуте d выглядит одинаково независимо от происхождения,
 * ни линтер, ни ревью его не опознают. Замок держит сигнатуры путей, убранных
 * при зачистке прав (Material Design Icons и Material Icons - Apache-2.0,
 * Feather - MIT): вернувшийся копипастой глиф валит тест с именем файла.
 *
 * Сигнатура - начало пути, достаточно длинное, чтобы не совпасть случайно.
 */
const FOREIGN_PATHS = [
  { set: 'Material Design Icons', glyph: 'shield-check', d: 'M12,3L4,6V11.1C4,15.6 7.4,19.8 12,21' },
  { set: 'Material Design Icons', glyph: 'office-building', d: 'M18,15H16V17H18M18,11H16V13H18M20,19H12' },
  { set: 'Material Design Icons', glyph: 'home-map-marker', d: 'M12,3L2,12H5V20H19V12H22L12,3M12,7.7' },
  { set: 'Material Design Icons', glyph: 'email', d: 'M22 6C22 4.9 21.1 4 20 4H4C2.9 4 2 4.9 2 6V18' },
  { set: 'Material Design Icons', glyph: 'phone', d: 'M6.62,10.79C8.06,13.62 10.38,15.94 13.21,17.38' },
  { set: 'Material Design Icons', glyph: 'lock', d: 'M12,17A2,2 0 0,0 14,15C14,13.89 13.1,13 12,13' },
  { set: 'Material Icons', glyph: 'info', d: 'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48' },
  { set: 'Material Icons', glyph: 'event', d: 'M19 3h-1V1h-2v2H8V1H6v2H5c-1.1 0-2 .9-2 2v14' },
  { set: 'Material Icons', glyph: 'schedule', d: 'M11.99 2C6.47 2 2 6.48 2 12s4.47 10 9.99 10' },
  { set: 'Feather', glyph: 'message-circle', d: 'M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7' },
  { set: 'Feather', glyph: 'eye-off', d: 'M3 3l18 18M10.6 6.1A11 11 0 0 1 12 6c6.5 0 10 6 10 6' },
  { set: 'Feather', glyph: 'settings', d: 'M19.4 13a7.8 7.8 0 0 0 0-2l2-1.5-2-3.5-2.4 1' },
  { set: 'Feather', glyph: 'paperclip', d: 'M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49' },
]

function collectVueFiles(dir, acc = []) {
  fs.readdirSync(dir, { withFileTypes: true }).forEach((entry) => {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      // Сами спеки пропускаем: сигнатуры лежат в этом же файле, иначе замок
      // ловил бы собственный перечень и падал бы всегда.
      if (entry.name !== 'node_modules' && entry.name !== '__tests__') collectVueFiles(full, acc)
    } else if (entry.name.endsWith('.vue') || entry.name.endsWith('.js')) {
      acc.push(full)
    }
  })
  return acc
}

describe('происхождение инлайновых иконок', () => {
  const srcDir = path.join(import.meta.dirname, '..', '..')
  const files = collectVueFiles(srcDir)

  it('находит исходники, по которым идёт проверка', () => {
    expect(files.length).toBeGreaterThan(100)
  })

  FOREIGN_PATHS.forEach(({ set, glyph, d }) => {
    it(`не содержит ${set} / ${glyph}`, () => {
      const guilty = files
        .filter((file) => fs.readFileSync(file, 'utf8').includes(d))
        .map((file) => path.relative(srcDir, file))
      expect(guilty, `путь ${glyph} из набора ${set} вернулся в разметку`).toEqual([])
    })
  })
})
