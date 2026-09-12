import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

/**
 * Карточки и истории сущностей переведены на общий лист (#2418, срез 4).
 *
 * Замок структурный: у этих экранов нет спек на выгрузку, монтировать каждый ради одной
 * кнопки дорого, а промахи тут ровно два - потеряли поле в вызове или оставили свою книгу.
 * Внешняя рамка проверяется отдельно у каждого: у истории входов её не было и раньше, и
 * «добавить всем на всякий случай» изменило бы файл, который человек уже привык видеть.
 */
const FILES = {
  'components/CarDetailsModal.vue': { outerBorder: true },
  'components/CreateApplication/VehicleDetailsModal.vue': { outerBorder: true },
  'components/CreateApplication/EmployeeDetailsModal.vue': { outerBorder: true },
  'components/CarHistoryModal.vue': { outerBorder: true },
  'components/CreateApplication/EmployeeHistoryModal.vue': { outerBorder: true },
  'components/ApplicationDetail/ApplicationHistory.vue': { outerBorder: true },
  'components/UserLoginHistory.vue': { outerBorder: false },
};

describe('карточки и истории сущностей выгружаются общим листом (#2418)', () => {
  for (const [file, expected] of Object.entries(FILES)) {
    it(`${file} зовёт общий лист со всеми полями`, () => {
      const text = readFileSync(join(process.cwd(), 'src', file), 'utf8');

      expect(text).toContain("from '@/utils/excelSheet'");
      expect(text).toContain('await downloadExcelSheet({');
      for (const field of ['sheetName:', 'header:', 'rows:', 'widths:', 'info:']) {
        expect(text, `${file}: нет поля ${field}`).toContain(field);
      }
      expect(text).toContain('Отчёт сформировал:');
      expect(text).toContain('Дата формирования:');

      // Ни своей книги, ни своих цветов и рамок - вид держит общий лист.
      expect(text).not.toContain('new ExcelJS.Workbook()');
      expect(text).not.toContain("fgColor: { argb: 'FF4F5BDF' }");
      expect(text).not.toContain("argb: 'FFE6E6E6'");

      if (expected.outerBorder) {
        expect(text, `${file}: потеряна внешняя рамка`).toContain('outerBorder: true');
      } else {
        expect(text, `${file}: рамки тут не было`).not.toContain('outerBorder: true');
      }
    });
  }

  // Имя файла истории заявки собирается из номера и организации и чистится от знаков,
  // недопустимых в файловой системе: переписать его «похоже» - значит переименовать файл,
  // который люди складывают в папки по годам.
  it('история заявки сохраняет прежнее имя файла', () => {
    const text = readFileSync(join(process.cwd(), 'src/components/ApplicationDetail/ApplicationHistory.vue'), 'utf8');
    expect(text).toContain('this.fileOrganizationName.replace');
    expect(text).toContain('`События_${safeAppNumber}_${safeOrgName}.xlsx`');
  });
});
