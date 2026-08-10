/**
 * Замок на рассинхрон языков (#1880).
 *
 * Словарь глаголов прав таблиц живёт в двух местах: Go генерирует ключи и
 * подписи (internal/services/permission_service.go, tableVerbs), фронт по этим
 * подписям разбирает display_name и группирует права по таблицам. Разойдутся --
 * новый глагол в Go тихо выпадет из группы и повиснет отдельной строкой секции.
 *
 * Сверка идёт с ВНЕШНИМ источником: одна сторона читается регуляркой из самого
 * Go-файла, а не из копии константы рядом.
 */
import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

import { TABLE_VERB_ORDER, TABLE_VERB_TITLES } from '../permissionCatalog';

const GO_FILE = path.resolve(__dirname, '../../../../internal/services/permission_service.go');

/** Пары [глагол, подпись] из объявления tableVerbs в Go, в порядке объявления. */
function goTableVerbs() {
  const src = fs.readFileSync(GO_FILE, 'utf8');
  const block = src.match(/var tableVerbs = \[\]struct\{[^}]*\}\{([\s\S]*?)\n\}/);
  if (!block) throw new Error('объявление tableVerbs не найдено в permission_service.go');
  return [...block[1].matchAll(/\{"([^"]+)",\s*"([^"]+)"\}/g)].map((m) => [m[1], m[2]]);
}

describe('словарь глаголов прав таблиц', () => {
  it('Go-файл читается и объявление разобрано', () => {
    expect(fs.existsSync(GO_FILE), GO_FILE).toBe(true);
    // Пустой разбор означал бы, что регулярка протухла, а не что глаголов нет:
    // без этой проверки сверка ниже стала бы тавтологией на пустых списках.
    expect(goTableVerbs().length).toBeGreaterThan(5);
  });

  it('состав и подписи совпадают с tableVerbs из Go', () => {
    expect(Object.entries(TABLE_VERB_TITLES)).toEqual(goTableVerbs());
  });

  it('порядок глаголов совпадает с порядком объявления в Go', () => {
    expect(TABLE_VERB_ORDER).toEqual(goTableVerbs().map(([verb]) => verb));
  });
});
