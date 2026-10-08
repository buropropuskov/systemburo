import { describe, expect, it, vi } from 'vitest';
import ExcelJS from 'exceljs';
import { buildExcelSheetBlob } from '../excelSheet';
import { forwardRecipientsText } from '../applicationForwardHistory';

vi.mock('@/utils/reportDownload', () => ({ downloadBlob: vi.fn() }));

async function readWorkbook(spec) {
  const blob = await buildExcelSheetBlob(spec);
  const buffer = typeof blob.arrayBuffer === 'function'
    ? await blob.arrayBuffer()
    : await new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(reader.result);
      reader.onerror = () => reject(reader.error);
      reader.readAsArrayBuffer(blob);
    });
  const workbook = new ExcelJS.Workbook();
  await workbook.xlsx.load(new Uint8Array(buffer));
  return workbook.getWorksheet('История');
}

describe('Реальный XLSX истории пересылки #2664', () => {
  it('сохраняет трёх получателей с переносом и достаточной высотой, не меняя число событий', async () => {
    const text = forwardRecipientsText({ recipient_details_available: true, recipient_details: [
      { user_id: 1, display_name: '@test_first', purpose: 'approval', required_approval: true, access_granted: true },
      { user_id: 2, display_name: '@test_second', purpose: 'approval', required_approval: false, access_granted: true },
      { user_id: 3, display_name: '@test_third', purpose: 'view', required_approval: false, access_granted: true },
    ] });
    const sheet = await readWorkbook({ sheetName: 'История', header: ['Действие', 'Получатели и назначения'], rows: [['Переслал(-а) заявку', text]], widths: [30, 60], wrapColumns: [1] });
    expect(sheet.rowCount).toBe(2);
    expect(sheet.getCell('B2').value).toBe(text);
    expect(sheet.getCell('B2').value.split('\n')).toHaveLength(3);
    expect(sheet.getCell('B2').alignment).toMatchObject({ vertical: 'top', wrapText: true });
    expect(sheet.getRow(2).height).toBeGreaterThanOrEqual(3 * 14 + 6);
    expect(sheet.getColumn(2).width).toBe(60);
    expect(sheet.getCell('A2').alignment.wrapText).toBeUndefined();
  });

  it('без опции оставляет прежнюю высоту и выравнивание остальных экспортов', async () => {
    const sheet = await readWorkbook({ sheetName: 'История', header: ['Текст'], rows: [['Первая\nВторая\nТретья']], widths: [60] });
    expect(sheet.getRow(2).height).toBe(20);
    expect(sheet.getCell('A2').alignment).toMatchObject({ vertical: 'middle' });
    expect(sheet.getCell('A2').alignment.wrapText).toBeUndefined();
  });

  it('соблюдает предел Excel, сохраняя полное значение ячейки', async () => {
    const text = Array.from({ length: 100 }, (_, index) => `@test_recipient_${index} — назначен согласующим`).join('\n');
    const sheet = await readWorkbook({ sheetName: 'История', header: ['Получатели'], rows: [[text]], widths: [60], wrapColumns: [0] });
    expect(sheet.getRow(2).height).toBe(409);
    expect(sheet.getCell('A2').value).toBe(text);
  });
});
