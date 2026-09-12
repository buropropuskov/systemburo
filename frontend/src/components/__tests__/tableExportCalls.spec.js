import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

/**
 * Выгрузки таблиц прохода переведены на общий лист (#2418, срез 3).
 *
 * Замок структурный: поведение выгрузки уже стерегут `CarsTable.bulkExport.spec.js` и
 * `PeopleTable.bulkExport.spec.js` (какие строки попали в книгу), а здесь проверяется то,
 * что они не видят - что вид листа берётся из общего модуля, а не собирается на месте, и
 * что в вызов передана внешняя рамка и подписи. Без этого файл молча уехал бы без рамки.
 */
const FILES = {
  'components/CarsTable.vue': { outerBorder: true, sheet: "'Avtomobili'" },
  'components/PeopleTable.vue': { outerBorder: true, sheet: "'Lyudi'" },
  'components/FactTable.vue': { outerBorder: true, sheet: 'sheetName,' },
  // Отчёт по проходам рамкой не обводился и раньше: у него подытоги внутри данных.
  'components/PassReportModal.vue': { outerBorder: false, sheet: "'Otchet_po_prohodam'" },
};

describe('таблицы прохода выгружаются общим листом (#2418)', () => {
  for (const [file, expected] of Object.entries(FILES)) {
    it(`${file} зовёт общий лист`, () => {
      const text = readFileSync(join(process.cwd(), 'src', file), 'utf8');

      expect(text).toContain("import { downloadExcelSheet } from '@/utils/excelSheet';");
      expect(text).toContain('await downloadExcelSheet({');
      expect(text).toContain(expected.sheet);
      expect(text).toContain('widths:');
      expect(text).toContain('Отчёт сформировал:');
      expect(text).toContain('Дата формирования:');

      // Своя книга больше не собирается - иначе вид разъедется с остальными выгрузками.
      expect(text).not.toContain('new ExcelJS.Workbook()');
      expect(text).not.toContain("fgColor: { argb: 'FF4F5BDF' }");

      if (expected.outerBorder) {
        expect(text, `${file}: потеряна внешняя рамка`).toContain('outerBorder: true');
      } else {
        expect(text).not.toContain('outerBorder: true');
      }
    });
  }

  // Подытог «Итого по посту» стоит внутри данных, и его полужирность держится списком
  // строк: потеряется список - подытоги сольются с обычными строками.
  it('отчёт по проходам отмечает подытоги полужирными', () => {
    const text = readFileSync(join(process.cwd(), 'src/components/PassReportModal.vue'), 'utf8');
    expect(text).toContain('Итого по посту');
    // Проверяем именно передачу списка в вызов: объявить его и забыть отдать - ровно тот
    // промах, от которого подытоги сольются с обычными строками.
    expect(text).toMatch(/rows,\s*\n\s*boldRows,/);
  });
});
