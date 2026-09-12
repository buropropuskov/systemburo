import { describe, it, expect, vi, beforeEach } from 'vitest';

/**
 * ExcelJS подменяем самописным двойником: настоящая библиотека собирает бинарный xlsx,
 * и проверить по нему вид листа нельзя - пришлось бы распаковывать книгу. Двойник же
 * запоминает, что именно ему велели нарисовать, и на этом стоят проверки вида (#2418).
 */
const sheets = [];
vi.mock('exceljs', () => {
  class Sheet {
    constructor(name) {
      this.name = name;
      this.rows = [];
      this.columns = [];
      this.cells = new Map();
    }

    addRow(values) {
      const cells = (values || []).map(value => ({ value, border: {} }));
      const row = {
        values,
        cells,
        height: null,
        eachCell(cb) { cells.forEach((cell, i) => cb(cell, i + 1)); },
      };
      this.rows.push(row);
      cells.forEach((cell, i) => this.cells.set(`${this.rows.length}:${i + 1}`, cell));
      return row;
    }

    getCell(row, col) {
      const key = `${row}:${col}`;
      if (!this.cells.has(key)) this.cells.set(key, { border: {} });
      return this.cells.get(key);
    }
  }

  class Workbook {
    constructor() {
      this.xlsx = { writeBuffer: async () => new Uint8Array([1, 2, 3]) };
    }

    addWorksheet(name) {
      const sheet = new Sheet(name);
      sheets.push(sheet);
      return sheet;
    }
  }

  return { default: { Workbook } };
});

const downloadBlob = vi.fn();
vi.mock('@/utils/reportDownload', () => ({ downloadBlob: (...args) => downloadBlob(...args) }));

import { buildExcelSheetBlob, downloadExcelSheet } from '../excelSheet';

const spec = () => ({
  sheetName: 'Istoriya',
  header: ['Дата и время', 'Пользователь'],
  rows: [['12.09.2026 10:00', 'Иванов И.И.'], ['12.09.2026 11:00', 'Петров П.П.']],
  widths: [25, 40],
  info: [['Отчёт сформировал:', 'Сидоров С.С.'], ['Дата формирования:', '12.09.2026 12:00']],
  outerBorder: true,
});

beforeEach(() => {
  sheets.length = 0;
  downloadBlob.mockReset();
});

describe('excelSheet - общий лист выгрузки', () => {
  it('рисует шапку, строки и подписи в привычном виде', async () => {
    await buildExcelSheetBlob(spec());
    const sheet = sheets[0];

    expect(sheet.name).toBe('Istoriya');
    // шапка + две строки + пустая + две подписи
    expect(sheet.rows).toHaveLength(6);

    const header = sheet.rows[0];
    expect(header.height).toBe(25);
    expect(header.cells[0].fill.fgColor.argb).toBe('FF4F5BDF');
    expect(header.cells[0].font).toMatchObject({ name: 'Verdana', size: 11, bold: true });

    // Чередование строк - тот же приём, что был в скопированных выгрузках.
    expect(sheet.rows[1].cells[0].fill.fgColor.argb).toBe('FFF0F5FF');
    expect(sheet.rows[2].cells[0].fill.fgColor.argb).toBe('FFE0E9FF');
    expect(sheet.rows[1].height).toBe(20);

    expect(sheet.rows[3].values).toEqual([]);
    expect(sheet.rows[4].values).toEqual(['Отчёт сформировал:', 'Сидоров С.С.']);
    // Подпись выравнивается влево явно: число Excel иначе прижимает вправо (#2332).
    expect(sheet.rows[4].cells[1].alignment).toMatchObject({ horizontal: 'left' });
  });

  it('ставит жирную внешнюю рамку только по просьбе', async () => {
    await buildExcelSheetBlob(spec());
    const bordered = sheets[0];
    expect(bordered.getCell(1, 1).border.left.style).toBe('medium');
    expect(bordered.getCell(3, 2).border.right.style).toBe('medium');
    expect(bordered.getCell(3, 1).border.bottom.style).toBe('medium');

    sheets.length = 0;
    await buildExcelSheetBlob({ ...spec(), outerBorder: false });
    expect(sheets[0].getCell(1, 1).border.left.style).toBe('thin');
  });

  it('заданные ширины берёт как есть, а без них считает по содержимому', async () => {
    await buildExcelSheetBlob(spec());
    expect(sheets[0].columns).toEqual([{ width: 25 }, { width: 40 }]);

    // Без своих ширин и без флага лист их не задаёт вовсе: у части выгрузок ширин не
    // было, и посчитать их «на всякий случай» значило бы изменить привычный файл.
    sheets.length = 0;
    await buildExcelSheetBlob({ ...spec(), widths: undefined });
    expect(sheets[0].columns).toEqual([]);

    sheets.length = 0;
    await buildExcelSheetBlob({ ...spec(), widths: undefined, autoWidths: true });
    const widths = sheets[0].columns.map(c => c.width);
    expect(widths).toHaveLength(2);
    // «Дата и время» под Verdana шире, чем 12 знаков, и не уже нижнего предела.
    expect(widths[0]).toBeGreaterThanOrEqual(20);
    expect(widths[1]).toBeLessThanOrEqual(60);
  });

  // Excel не открывает книгу с именем листа длиннее 31 знака, а имена собираются из
  // данных («Istoriya_<название организации>»), поэтому обрезаем сами.
  it('обрезает длинное имя листа', async () => {
    await buildExcelSheetBlob({ ...spec(), sheetName: 'Istoriya_очень_длинное_название_организации_и_ещё' });
    expect(sheets[0].name).toHaveLength(31);
  });

  it('строку итогов рисует своим цветом и включает в рамку', async () => {
    await buildExcelSheetBlob({ ...spec(), totalsRow: ['Итого', 2] });
    const sheet = sheets[0];
    const totals = sheet.rows[3];
    expect(totals.values).toEqual(['Итого', 2]);
    expect(totals.cells[0].fill.fgColor.argb).toBe('FFD3DCFF');
    expect(sheet.getCell(4, 1).border.bottom.style).toBe('medium');
  });

  // Подытог внутри данных (отчёт по проходам): «Итого по посту» стоит после строк своего
  // дня, а не отдельной строкой в конце, поэтому у листа есть список полужирных строк.
  it('полужирными набирает только указанные строки данных', async () => {
    await buildExcelSheetBlob({ ...spec(), boldRows: [1] });
    const sheet = sheets[0];
    expect(sheet.rows[1].cells[0].font.bold).toBe(false);
    expect(sheet.rows[2].cells[0].font.bold).toBe(true);
  });

  it('скачивание отдаёт файл через общую утилиту', async () => {
    await downloadExcelSheet(spec(), 'Istoriya_12-09-2026.xlsx');
    expect(downloadBlob).toHaveBeenCalledTimes(1);
    expect(downloadBlob.mock.calls[0][1]).toBe('Istoriya_12-09-2026.xlsx');
  });
});
