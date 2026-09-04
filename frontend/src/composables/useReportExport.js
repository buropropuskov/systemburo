import { ref } from 'vue';
import { reportToTable } from '@/utils/reportTable';
import { downloadBlob, downloadFileName } from '@/utils/reportDownload';
import { exportExcel } from '@/utils/reportExcel';
import { exportPdf } from '@/utils/reportPdf';

/**
 * Сохранение графика картинкой. Графики рисуются на canvas, поэтому кадр берётся
 * прямо из него - пересчитывать данные в свою отрисовку незачем.
 *
 * @param {HTMLCanvasElement} canvasEl холст графика
 * @param {{ title?: string }} [opts] подпись для имени файла
 * @returns {Promise<void>}
 */
export function exportChartPng(canvasEl, opts = {}) {
  return new Promise((resolve, reject) => {
    if (!canvasEl?.toBlob) {
      reject(new Error('График не найден'));
      return;
    }
    canvasEl.toBlob((blob) => {
      if (!blob) {
        reject(new Error('Не удалось сохранить график'));
        return;
      }
      downloadBlob(blob, downloadFileName(opts, 'png'));
      resolve();
    }, 'image/png');
  });
}

/**
 * Выгрузка результата отчёта в выбранном формате. ExcelJS/pdfmake грузятся лениво.
 */
export function useReportExport() {
  const exporting = ref(false);

  /**
   * @param {object} result результат отчёта
   * @param {{ title?: string, period?: {from?: string, to?: string}, author?: string }} [opts]
   * @param {'excel'|'pdf'} [format] формат выгрузки
   */
  async function exportReport(result, opts = {}, format = 'excel') {
    const table = reportToTable(result);
    if (!table.header.length) throw new Error('Нет данных для выгрузки');

    exporting.value = true;
    try {
      if (format === 'pdf') {
        await exportPdf(table, opts);
      } else {
        await exportExcel(table, opts);
      }
    } finally {
      exporting.value = false;
    }
  }

  return { exporting, exportReport };
}
