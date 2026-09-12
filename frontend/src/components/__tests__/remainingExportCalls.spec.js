import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

/**
 * Последние выгрузки переведены на общий лист (#2418, срез 5).
 *
 * У истории чёрного списка набор колонок непостоянный - «Объект» появляется только у
 * разделов с подписью сущности, - поэтому заголовки там берутся из самих строк, а ширины
 * из словаря. Это стоит отдельной проверки: подставить фиксированный список колонок
 * значило бы потерять или сдвинуть колонку в половине разделов.
 */
const FILES = {
  'components/SystemTableHistoryModal.vue': { preset: 'directory', outerBorder: null },
  'components/ApplicationApproverHistoryModal.vue': { preset: 'sheet', outerBorder: true },
  'components/TrashHistoryModal.vue': { preset: 'sheet', outerBorder: false },
  'views/TrashView.vue': { preset: 'sheet', outerBorder: false },
  'components/admin/blacklist/BlacklistHistoryModalBase.vue': { preset: 'sheet', outerBorder: true },
};

describe('остальные выгрузки переведены на общий лист (#2418)', () => {
  for (const [file, expected] of Object.entries(FILES)) {
    it(`${file} не собирает свою книгу`, () => {
      const text = readFileSync(join(process.cwd(), 'src', file), 'utf8');

      expect(text).not.toContain('new ExcelJS.Workbook()');
      expect(text).not.toContain("fgColor: { argb: 'FF4F5BDF' }");
      if (expected.preset === 'directory') {
        expect(text).toContain('await downloadDirectoryHistory({');
      } else {
        expect(text).toContain('await downloadExcelSheet({');
        if (expected.outerBorder) {
          expect(text, `${file}: потеряна внешняя рамка`).toContain('outerBorder: true');
        } else {
          expect(text, `${file}: рамки тут не было`).not.toContain('outerBorder: true');
        }
      }
    });
  }

  it('история чёрного списка берёт колонки из данных, а ширины из словаря', () => {
    const text = readFileSync(join(process.cwd(), 'src/components/admin/blacklist/BlacklistHistoryModalBase.vue'), 'utf8');
    expect(text).toContain('const header = Object.keys(data[0]);');
    expect(text).toContain('BLACKLIST_HISTORY_WIDTHS[column]');
    // Колонка «Объект» - та самая непостоянная: если заголовки перестанут строиться по
    // данным, у половины разделов колонки сдвинутся.
    expect(text).toContain('Объект');
  });

  // Выгрузка корзины ширин не задавала - общий лист не должен их придумывать сам.
  it('выгрузка корзины остаётся без заданных ширин', () => {
    const text = readFileSync(join(process.cwd(), 'src/views/TrashView.vue'), 'utf8');
    const call = text.slice(text.indexOf('await downloadExcelSheet({'), text.indexOf('}, `korzina_'));
    expect(call).not.toContain('widths:');
    expect(call).not.toContain('autoWidths');
  });
});
