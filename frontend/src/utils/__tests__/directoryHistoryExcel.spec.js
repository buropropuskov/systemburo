import { describe, it, expect, vi, beforeEach } from 'vitest';

const downloadExcelSheet = vi.fn();
vi.mock('@/utils/excelSheet', () => ({
  downloadExcelSheet: (...args) => downloadExcelSheet(...args),
}));

import {
  DIRECTORY_HISTORY_HEADER,
  DIRECTORY_HISTORY_WIDTHS,
  downloadDirectoryHistory,
} from '../directoryHistoryExcel';

beforeEach(() => {
  downloadExcelSheet.mockReset();
});

// Девять модалок истории справочника выгружали историю одинаково, по сто строк кода на
// каждую (#2418). Пресет держит колонки и ширины в одном месте - и эти значения должны
// остаться теми же, иначе привычный файл поедет.
describe('directoryHistoryExcel - пресет истории справочника', () => {
  it('колонки и ширины совпадают с прежними выгрузками', () => {
    expect(DIRECTORY_HISTORY_HEADER).toEqual([
      'Дата и время', 'Пользователь', 'Действие', 'Детали', 'Тип действия', 'ID записи',
    ]);
    expect(DIRECTORY_HISTORY_WIDTHS).toEqual([22, 30, 30, 60, 22, 12]);
  });

  it('передаёт листу строки, подписи и внешнюю рамку', async () => {
    await downloadDirectoryHistory({
      sheetName: 'Istoriya_RF',
      filename: 'Istoriya_grazhdanstva_RF_12-09-2026.xlsx',
      rows: [['12.09.2026 10:00', 'Иванов И.И.', 'Изменено', 'Название', 'update', 7]],
      author: 'Сидоров С.С.',
      formedAt: '12.09.2026 12:00',
    });

    expect(downloadExcelSheet).toHaveBeenCalledTimes(1);
    const [spec, filename] = downloadExcelSheet.mock.calls[0];
    expect(filename).toBe('Istoriya_grazhdanstva_RF_12-09-2026.xlsx');
    expect(spec).toMatchObject({
      sheetName: 'Istoriya_RF',
      header: DIRECTORY_HISTORY_HEADER,
      widths: DIRECTORY_HISTORY_WIDTHS,
      outerBorder: true,
    });
    expect(spec.rows).toHaveLength(1);
    expect(spec.info).toEqual([
      ['Отчёт сформировал:', 'Сидоров С.С.'],
      ['Дата формирования:', '12.09.2026 12:00'],
    ]);
  });
});
