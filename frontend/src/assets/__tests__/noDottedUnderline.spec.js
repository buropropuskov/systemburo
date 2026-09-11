import { describe, it, expect } from 'vitest';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, resolve, relative } from 'node:path';

/**
 * Запрет пунктирных подчёркиваний во всём интерфейсе.
 *
 * Владелец забраковал их дважды - у сводки отметок исполнения и у кнопки «Заметка
 * бюро», - и просил больше так не делать. Нажимаемость текстовой кнопки показывают
 * цвет, ховер и курсор, а не пунктир под строкой.
 *
 * Замок общий, а не на два файла: иначе тот же приём вернётся в третьем месте.
 */
const ROOT = resolve(process.cwd(), 'src');

function walk(dir, acc = []) {
  for (const name of readdirSync(dir)) {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) {
      if (name === '__tests__') continue;
      walk(full, acc);
    } else if (/\.(vue|css)$/.test(name)) {
      acc.push(full);
    }
  }
  return acc;
}

describe('оформление: без пунктирных подчёркиваний', () => {
  it('ни один файл не подчёркивает текст пунктиром', () => {
    const guilty = [];
    for (const file of walk(ROOT)) {
      const text = readFileSync(file, 'utf8').replace(/\/\*[\s\S]*?\*\//g, '');
      if (/text-decoration[^;]*\b(dotted|dashed)\b/.test(text)
        || /text-decoration-style:\s*(dotted|dashed)/.test(text)) {
        guilty.push(relative(ROOT, file));
      }
    }
    expect(guilty, `пунктирное подчёркивание запрещено: ${guilty.join(', ')}`).toEqual([]);
  });
});
