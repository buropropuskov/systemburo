// @vitest-environment node
import { createRequire } from 'node:module';
import { describe, expect, it } from 'vitest';
import JSZip from 'jszip';

const require = createRequire(import.meta.url);
const excelRequire = createRequire(require.resolve('exceljs'));
const uuid = excelRequire('uuid');
const factories = [
  ['CommonJS', require('exceljs')],
  // Vite берёт готовую browser-сборку ExcelJS, а не его CommonJS-вход.
  ['browser', require('exceljs/dist/exceljs.min.js')],
];

describe('совместимость ExcelJS и его зависимости uuid', () => {
  it('сохраняет CommonJS-интерфейс v4, используемый ExcelJS', () => {
    const identifiers = Array.from({ length: 20 }, () => uuid.v4());
    expect(new Set(identifiers).size).toBe(identifiers.length);
    for (const identifier of identifiers) {
      expect(uuid.validate(identifier)).toBe(true);
      expect(uuid.version(identifier)).toBe(4);
    }
  });

  it.each(factories)('%s: записывает и читает XLSX, включая ветку с uuid', async (_name, ExcelJS) => {
    const workbook = new ExcelJS.Workbook();
    const sheet = workbook.addWorksheet('Проверка экспорта');
    sheet.columns = [{ width: 35 }, { width: 18 }];
    sheet.addRow(['Описание', 'Количество']);
    sheet.addRow(['Кириллица и перенос\nвторой строки', 12]);
    sheet.addRow(['Вторая запись', 8]);
    sheet.addRow(['Итого', { formula: 'SUM(B2:B3)', result: 20 }]);
    sheet.getCell('A1').font = { name: 'Verdana', bold: true };
    sheet.getCell('A2').alignment = { wrapText: true };
    sheet.views = [{ state: 'frozen', ySplit: 1 }];
    sheet.addConditionalFormatting({ ref: 'B2:B3', rules: [{
      type: 'dataBar', gradient: false,
      cfvo: [{ type: 'min' }, { type: 'max' }],
      color: { argb: 'FF4F5BDF' },
    }] });
    const bytes = await workbook.xlsx.writeBuffer();
    const zip = await JSZip.loadAsync(bytes);
    const xml = await zip.file('xl/worksheets/sheet1.xml').async('string');
    const ids = [...xml.matchAll(/\{([A-F0-9-]{36})\}/g)].map(match => match[1]);
    expect(ids.length).toBeGreaterThan(0);
    for (const id of ids) {
      expect(uuid.validate(id)).toBe(true);
      expect(uuid.version(id)).toBe(4);
    }
    const restored = new ExcelJS.Workbook();
    await restored.xlsx.load(bytes);
    const result = restored.getWorksheet('Проверка экспорта');
    expect(result.rowCount).toBe(4);
    expect(result.getCell('A2').value).toBe('Кириллица и перенос\nвторой строки');
    expect(result.getCell('B2').value).toBe(12);
    expect(result.getCell('B4').value).toEqual({ formula: 'SUM(B2:B3)', result: 20 });
    expect(result.getCell('A1').font).toMatchObject({ name: 'Verdana', bold: true });
    expect(result.getCell('A2').alignment.wrapText).toBe(true);
    expect(result.getColumn(1).width).toBe(35);
    expect(result.views[0]).toMatchObject({ state: 'frozen', ySplit: 1 });
  });
});
