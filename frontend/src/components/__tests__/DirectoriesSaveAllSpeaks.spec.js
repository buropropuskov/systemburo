import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { join, resolve } from 'node:path';

/**
 * «Сохранить все изменения» не должно молчать на неполной форме (#871).
 *
 * Кнопка создания заблокирована, пока форма не заполнена, поэтому невалидный
 * вызов submitAdd приходит с другой стороны: из диалога несохранённых изменений,
 * который вызывает save() трекера. Тихий `return` там выглядит как «нажал, и
 * ничего не произошло» - saveAllDirty видит, что форма осталась грязной
 * (dirtyTracker.js: `if (entry.isDirty()) return false`), и просто не закрывает
 * диалог, ничего не объясняя. Ровно это и поймал владелец на организациях:
 * ввёл название, не выбрал тип, нажал сохранить - тишина.
 *
 * Замок текстовый: поднимать пять компонентов с их сторами ради одной ветки
 * дороже, чем сверить, что в submitAdd перед выходом заполняется addError.
 */

const componentsDir = resolve(__dirname, '..');

const DIRECTORIES = [
  'OrganizationsManagement',
  'CompaniesManagement',
  'CitizenshipManagement',
  'MarksManagement',
  'AttachmentsManagement',
];

/** Тело submitAdd от объявления до первого this.isAdding = true. */
function submitAddHead(source) {
  const start = source.indexOf('async submitAdd()');
  if (start < 0) return null;
  const end = source.indexOf('this.isAdding = true', start);
  if (end < 0) return null;
  return source.slice(start, end);
}

describe('справочники: неполная форма объясняет причину, а не молчит', () => {
  it.each(DIRECTORIES)('%s: submitAdd не выходит молча', (name) => {
    const source = readFileSync(join(componentsDir, `${name}.vue`), 'utf8');
    const head = submitAddHead(source);
    expect(head, `${name}: submitAdd не найден`).toBeTruthy();

    // Ранние выходы, кроме защиты от повторного нажатия.
    const guards = [...head.matchAll(/if \(([^)]*)\) \{?\s*(?:\n[^}]*)?\breturn\b/g)]
      .map((m) => m[1].trim())
      .filter((cond) => cond !== 'this.isAdding');

    for (const cond of guards) {
      const branch = head.slice(head.indexOf(cond));
      const untilReturn = branch.slice(0, branch.indexOf('return') + 6);
      expect(
        untilReturn.includes('this.addError'),
        `${name}: выход по «${cond}» ничего не сообщает - из диалога сохранения это выглядит как зависшая кнопка`,
      ).toBe(true);
    }
  });

  it.each(DIRECTORIES)('%s: Escape не обрабатывается в обход BaseModal', (name) => {
    const source = readFileSync(join(componentsDir, `${name}.vue`), 'utf8');
    expect(
      source.includes("addEventListener('keydown'"),
      `${name}: свой слушатель Escape поверх BaseModal - тот ведёт стек модалок и знает, какое окно сверху`,
    ).toBe(false);
  });
});
