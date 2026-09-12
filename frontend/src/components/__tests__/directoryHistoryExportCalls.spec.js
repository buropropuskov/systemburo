import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

/**
 * Девять модалок истории справочника переведены на общий пресет (#2418, срез 1).
 *
 * Замок структурный, а не поведенческий: у этих модалок нет спек, монтировать каждую ради
 * одной кнопки дорого, а дефект тут возможен ровно один - забыли передать что-нибудь из
 * пяти полей, и выгрузка молча уезжает без имени файла или без подписи.
 */
const MODALS = [
  'components/CitizenshipHistoryModal.vue',
  'components/CompanyHistoryModal.vue',
  'components/OrgHistoryModal.vue',
  'components/UserTypeHistoryModal.vue',
  'components/MarkHistoryModal.vue',
  'components/LicensePlateFormatHistoryModal.vue',
  'components/UniqueAttachmentHistoryModal.vue',
  'components/UserHistoryModal.vue',
  'components/UnloadPlaces/UnloadPlaceHistoryModal.vue',
];

const REQUIRED = ['sheetName:', 'filename:', 'rows:', 'author:', 'formedAt:'];

describe('истории справочников выгружаются общим пресетом (#2418)', () => {
  for (const file of MODALS) {
    it(`${file} вызывает пресет со всеми полями`, () => {
      const text = readFileSync(join(process.cwd(), 'src', file), 'utf8');
      expect(text).toContain("import { downloadDirectoryHistory } from '@/utils/directoryHistoryExcel';");
      expect(text).toContain('await downloadDirectoryHistory({');
      for (const field of REQUIRED) {
        expect(text, `${file}: нет поля ${field}`).toContain(field);
      }
      // Своя книга и собственный список колонок больше не нужны - иначе вид разъедется.
      expect(text).not.toContain('new ExcelJS.Workbook()');
      expect(text).not.toContain("'Тип действия'");
      // Сбой выгрузки по-прежнему виден человеку, а не только в консоли.
      expect(text).toContain('выгрузить историю');
    });
  }
});
