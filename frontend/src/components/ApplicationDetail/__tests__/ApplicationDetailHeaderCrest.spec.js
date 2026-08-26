import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, it, expect } from 'vitest';

const SFC = readFileSync(resolve(__dirname, '../ApplicationDetail.vue'), 'utf8');

/** Тело правила для селектора, без учёта переносов. jsdom раскладку не считает,
 *  поэтому CSS-контракт стережём чтением исходника, а не рендером (см. паттерн
 *  в ApplicationAttachmentSupplementMarks.spec.js). */
function rule(src, selector) {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const found = src.match(new RegExp(`${escaped}\\s*\\{([^}]*)\\}`));
  return found ? found[1].replace(/\s+/g, ' ').trim() : null;
}

// Регрессия шапки детали заявки (#1685 -> дефект владельца, замерено в браузере):
// ряд действий по дополнению сделал .action-bar-root двухрядным, и .detail-header
// центрирует .detail-header-left/.detail-header-right по высоте (align-items:
// center) - крестик уезжал вниз на 30-157px в зависимости от ширины окна.
// closeTop гулял 93.8-201.8 при неизменном cardTop=45 ДО фикса, стал константой
// 60/60 на сетке 769-1440 ПОСЛЕ.
describe('ApplicationDetail - крестик закрытия прижат к верху шапки (#1685 регрессия)', () => {
  it('крестик прижат к верху своего ряда', () => {
    expect(rule(SFC, '.close-detail-btn')).toMatch(/align-self:\s*flex-start/);
  });

  it('правая часть шапки прижата к верху всей шапки (детали центрируются в .detail-header)', () => {
    expect(rule(SFC, '.detail-header-right')).toMatch(/align-self:\s*flex-start/);
  });

  // nowrap на этом уровне обязателен: если бы переносились бейдж+action-bar+крестик
  // ВМЕСТЕ, крестик мог уйти на свою отдельную строку и уехать вниз ещё раз -
  // так и было при первом заходе фикса (closeTop 159.6 на ширине 900).
  it('крестик не участвует в переносе бейджа и action-bar - вынесены в свою обёртку', () => {
    expect(rule(SFC, '.detail-header-right')).toMatch(/flex-wrap:\s*nowrap/);
    expect(rule(SFC, '.detail-header-actions')).toMatch(/flex-wrap:\s*wrap/);
  });
});
