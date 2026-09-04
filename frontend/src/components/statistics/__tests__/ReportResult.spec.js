import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { nextTick } from 'vue';
import ReportResult from '../ReportResult.vue';

// Экспорт в Excel тянет ExcelJS и пишет файл — в юнит-тесте подменяем composable
// и проверяем, что клик «Excel» зовёт exportReport с результатом и meta.
const { exportSpy, pngSpy } = vi.hoisted(() => ({ exportSpy: vi.fn(), pngSpy: vi.fn() }));
vi.mock('@/composables/useReportExport', async () => {
  const { ref } = await import('vue');
  return {
    useReportExport: () => ({ exporting: ref(false), exportReport: exportSpy }),
    exportChartPng: pngSpy,
  };
});

// Графики требуют настоящего холста или SVG с измерениями — в юнит-тесте
// переключателя они не нужны, подменяем стабами и читаем переданные props.
const areaStub = {
  name: 'AnalyticsAreaChart',
  props: ['data', 'height', 'seriesName', 'unitForms', 'isFloat', 'valueType', 'tension'],
  template: '<div class="area-stub" />',
};
const barStub = {
  name: 'AnalyticsBarChart',
  props: ['data', 'height', 'seriesName', 'unitForms', 'isFloat', 'valueType'],
  template: '<div class="bar-stub" />',
};
const donutStub = {
  name: 'AnalyticsDonutChart',
  props: ['data', 'height', 'totalLabel', 'unitForms', 'isFloat'],
  template: '<div class="donut-stub" />',
};

function mountResult(result) {
  return mount(ReportResult, {
    props: { result },
    global: {
      stubs: { AnalyticsAreaChart: areaStub, AnalyticsBarChart: barStub, AnalyticsDonutChart: donutStub },
    },
  });
}

const aggPeriod = {
  mode: 'aggregate', dimension: 'period', unit: 'шт',
  rows: [{ label: '2026-06-01', value: 2 }], total: 2,
};
const aggStatus = {
  mode: 'aggregate', dimension: 'status', unit: 'шт',
  rows: [{ label: 'Завершено', value: 5 }], total: 5,
};
const listRes = { mode: 'list', columns: [{ key: 'a', label: 'A' }], rows: [{ a: 'x' }], total: 1 };

// Мультиметрика (формат движка GR0): columns + metric_rows{label,values} + totals.
const aggMulti = {
  mode: 'aggregate',
  dimension: 'organization',
  columns: [
    { key: 'applications_count', label: 'Количество заявок', unit: 'шт' },
    { key: 'items_sum', label: 'Количество товаров', unit: 'шт' },
  ],
  metric_rows: [
    { label: 'ООО А', values: { applications_count: 10, items_sum: 120 } },
    { label: 'ООО Б', values: { applications_count: 4, items_sum: 30 } },
  ],
  totals: { applications_count: 14, items_sum: 150 },
};
const aggNone = {
  mode: 'aggregate',
  dimension: 'none',
  columns: [{ key: 'applications_count', label: 'Количество заявок', unit: 'шт' }],
  metric_rows: [{ label: 'Итого', values: { applications_count: 14 } }],
  totals: { applications_count: 14 },
};

// Cross-tab pivot + дробная метрика: период-разрез, обычная метрика + float-колонка
// (среднее, значение в float_values/float_totals) + pivot-колонки (значение в values).
const aggPivotFloat = {
  mode: 'aggregate',
  dimension: 'period',
  columns: [
    { key: 'car_entries_count', label: 'Машины', unit: 'шт', kind: 'metric' },
    { key: 'avg_cars_per_day', label: 'Среднее/день', unit: 'шт/день', kind: 'metric', float: true },
    { key: 'att_propusk', label: 'Пропуск', kind: 'pivot' },
    { key: 'att_zayavka', label: 'Заявка', kind: 'pivot' },
  ],
  metric_rows: [
    {
      label: '2026-06-01',
      values: { car_entries_count: 10, att_propusk: 6, att_zayavka: 4 },
      float_values: { avg_cars_per_day: 2.5 },
    },
  ],
  totals: { car_entries_count: 10, att_propusk: 6, att_zayavka: 4 },
  float_totals: { avg_cars_per_day: 2.5 },
};

// Метрики обработки заявок (#1240): длительность — целые СЕКУНДЫ в values с
// type='duration', доля — дробь в float_values с float=true. Форма фикстуры взята с
// бэка (ReportMetricColumn.Type/Float, applyRateScale). У «ООО Б» этап завершения не
// пройден и доля не считана — движок ключ не выставляет («нет данных» != 0).
const aggDuration = {
  mode: 'aggregate',
  dimension: 'organization',
  columns: [
    { key: 'avg_approval_time', label: 'Время согласования', type: 'duration' },
    { key: 'avg_completion_time', label: 'Время до завершения', type: 'duration' },
    { key: 'refusal_rate', label: 'Доля отказов', unit: '%', float: true },
    { key: 'applications_count', label: 'Количество заявок', unit: 'шт' },
  ],
  metric_rows: [
    {
      label: 'ООО А',
      values: { avg_approval_time: 8100, avg_completion_time: 259200, applications_count: 10 },
      float_values: { refusal_rate: 12.5 },
    },
    {
      label: 'ООО Б',
      values: { avg_approval_time: 0, applications_count: 4 },
      float_values: {},
    },
  ],
  totals: { avg_approval_time: 5400, avg_completion_time: 259200, applications_count: 14 },
  float_totals: { refusal_rate: 12.5 },
};

describe('ReportResult — переключатель Таблица/График', () => {
  it('list-режим: переключателя нет, только таблица', () => {
    const w = mountResult(listRes);
    expect(w.find('[data-testid="rr-view-chart"]').exists()).toBe(false);
    expect(w.find('.rr__table').exists()).toBe(true);
  });

  it('list-режим: колонки date/time форматируются, текст с датой — нет', () => {
    const w = mountResult({
      mode: 'list',
      columns: [
        { key: 'period', label: 'Период работ', type: 'date' },
        { key: 'time', label: 'Время работ', type: 'time' },
        { key: 'name', label: 'Наименование работ' }, // без типа — свободный текст
      ],
      rows: [{ period: '2026-06-20 - 2026-06-21', time: '00:01:00 - 23:59:00', name: 'Ремонт 2026-06-15' }],
      total: 1,
    });
    const cells = w.findAll('.rr__table tbody tr')[0].findAll('td').map((td) => td.text());
    expect(cells[0]).toBe('20.06.2026 - 21.06.2026');
    expect(cells[1]).toBe('00:01 - 23:59');
    expect(cells[2]).toBe('Ремонт 2026-06-15'); // дата в тексте не тронута
  });

  it('list-режим: числовые колонки получают rr__num (right), текст/идентификаторы — нет', () => {
    const w = mountResult({
      mode: 'list',
      columns: [
        { key: 'number', label: 'Номер заявки' }, // строка-идентификатор -> текст
        { key: 'status', label: 'Статус' }, // текст
        { key: 'attachments_count', label: 'Вложений' }, // число -> right
        { key: 'people_count', label: 'Кол-во людей' }, // число (включая 0) -> right
      ],
      rows: [
        { number: '2026-001', status: 'Завершено', attachments_count: 3, people_count: 12 },
        { number: '2026-002', status: 'В работе', attachments_count: 0, people_count: 5 },
      ],
      total: 2,
    });
    const headers = w.findAll('.rr__table thead th');
    expect(headers[0].classes()).not.toContain('rr__num'); // Номер заявки — строка
    expect(headers[1].classes()).not.toContain('rr__num'); // Статус
    expect(headers[2].classes()).toContain('rr__num'); // Вложений (включая 0 во 2-й строке)
    expect(headers[3].classes()).toContain('rr__num'); // Кол-во людей

    expect(w.findAll('.rr__table tbody tr')[0].findAll('td')[0].classes()).not.toContain('rr__num');
    // Ячейка со значением 0 (2-я строка, Вложений) тоже выровнена — 0 не считается пустым.
    expect(w.findAll('.rr__table tbody tr')[1].findAll('td')[2].classes()).toContain('rr__num');
  });

  it('list-режим: колонка целиком из пустых значений не считается числовой', () => {
    const w = mountResult({
      mode: 'list',
      columns: [
        { key: 'count', label: 'Вложений' }, // все значения null/пусто -> не числовая
        { key: 'name', label: 'Наименование' },
      ],
      rows: [
        { count: null, name: 'Ремонт' },
        { count: '', name: 'Уборка' },
      ],
      total: 2,
    });
    const headers = w.findAll('.rr__table thead th');
    expect(headers[0].classes()).not.toContain('rr__num');
    expect(headers[1].classes()).not.toContain('rr__num');
  });

  it('aggregate: переключатель есть, по умолчанию таблица', () => {
    const w = mountResult(aggStatus);
    expect(w.find('[data-testid="rr-view-chart"]').exists()).toBe(true);
    expect(w.find('.rr__table').exists()).toBe(true);
    expect(w.findComponent(barStub).exists()).toBe(false);
    expect(w.findComponent(areaStub).exists()).toBe(false);
  });

  it('aggregate период: график — area (динамика во времени)', async () => {
    const w = mountResult(aggPeriod);
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();
    expect(w.findComponent(areaStub).exists()).toBe(true);
    expect(w.findComponent(barStub).exists()).toBe(false);
    // area получает ISO-подпись в timestamp — компонент сам форматит дд.мм.
    expect(w.findComponent(areaStub).props('data')).toEqual([{ timestamp: '2026-06-01', count: 2 }]);
  });

  it('aggregate разрез (не период): график — столбцы', async () => {
    const w = mountResult(aggStatus);
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();
    expect(w.findComponent(barStub).exists()).toBe(true);
    expect(w.findComponent(areaStub).exists()).toBe(false);
    expect(w.findComponent(barStub).props('data')).toEqual([{ label: 'Завершено', value: 5 }]);
  });

  it('aggregate с 0 строк: кнопка График недоступна', () => {
    const w = mountResult({ ...aggStatus, rows: [], total: 0 });
    expect(w.find('[data-testid="rr-view-chart"]').attributes('disabled')).toBeDefined();
  });

  it('в режиме график пришёл пустой результат: график убран, таблица и блокировка кнопки', async () => {
    const w = mountResult(aggStatus);
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();
    expect(w.findComponent(barStub).exists()).toBe(true);

    await w.setProps({ result: { ...aggStatus, rows: [], total: 0 } });
    await nextTick();
    expect(w.findComponent(barStub).exists()).toBe(false);
    expect(w.find('.rr__table').exists()).toBe(true);
    expect(w.find('[data-testid="rr-view-chart"]').attributes('disabled')).toBeDefined();
  });

  it('период-строки выводятся как дд.мм.гггг', () => {
    const w = mountResult(aggPeriod);
    const firstCell = w.findAll('.rr__table tbody tr')[0].findAll('td')[0].text();
    expect(firstCell).toBe('01.06.2026');
  });

  it('мультиметрика: колонка на метрику, значения по строкам и итоговая строка', () => {
    const w = mountResult(aggMulti);
    const headers = w.findAll('.rr__table thead th').map((th) => th.text());
    expect(headers[0]).toBe('Значение разреза');
    expect(headers[1]).toContain('Количество заявок');
    expect(headers[2]).toContain('Количество товаров');

    const firstRow = w.findAll('.rr__table tbody tr')[0].findAll('td').map((td) => td.text());
    expect(firstRow).toEqual(['ООО А', '10', '120']);

    const footRow = w.findAll('.rr__table tfoot td').map((td) => td.text());
    expect(footRow).toEqual(['Итого', '14', '150']);
  });

  it('cross-tab pivot + float: pivot-колонки в values, float — в float_values/float_totals', () => {
    const w = mountResult(aggPivotFloat);
    const headers = w.findAll('.rr__table thead th').map((th) => th.text());
    expect(headers).toEqual(['Значение разреза', 'Машины, шт', 'Среднее/день, шт/день', 'Пропуск', 'Заявка']);

    const row = w.findAll('.rr__table tbody tr')[0].findAll('td').map((td) => td.text());
    // дата + счётчик + float (с дробной) + два pivot-счётчика
    expect(row).toEqual(['01.06.2026', '10', '2,5', '6', '4']);

    const foot = w.findAll('.rr__table tfoot td').map((td) => td.text());
    expect(foot).toEqual(['Итого', '10', '2,5', '6', '4']);
  });

  it('cross-tab: график float-метрики берёт значение из float_values', async () => {
    const w = mountResult(aggPivotFloat);
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await w.find('[data-testid="rr-metric-avg_cars_per_day"]').trigger('click');
    await nextTick();
    expect(w.findComponent(areaStub).props('data')).toEqual([{ timestamp: '2026-06-01', count: 2.5 }]);
    expect(w.findComponent(areaStub).props('seriesName')).toBe('Среднее/день');
    // float-метрика -> график не округляет (тултип/ось в дробях, как таблица).
    expect(w.findComponent(areaStub).props('isFloat')).toBe(true);
  });

  it('целочисленная метрика: isFloat=false (график округляет до целых)', async () => {
    const w = mountResult(aggPeriod);
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();
    expect(w.findComponent(areaStub).props('isFloat')).toBe(false);
  });

  it('мультиметрика: график показывает селектор метрик и рисует выбранную', async () => {
    const w = mountResult(aggMulti);
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();

    // По умолчанию первая метрика (organization -> bar).
    expect(w.findComponent(barStub).props('data')).toEqual([
      { label: 'ООО А', value: 10 }, { label: 'ООО Б', value: 4 },
    ]);
    expect(w.findComponent(barStub).props('seriesName')).toBe('Количество заявок');

    // Переключаем на вторую метрику.
    await w.find('[data-testid="rr-metric-items_sum"]').trigger('click');
    await nextTick();
    expect(w.findComponent(barStub).props('data')).toEqual([
      { label: 'ООО А', value: 120 }, { label: 'ООО Б', value: 30 },
    ]);
    expect(w.findComponent(barStub).props('seriesName')).toBe('Количество товаров');
  });

  it('разрез «без разреза»: заголовок «Итог», без отдельной строки итогов', () => {
    const w = mountResult(aggNone);
    expect(w.findAll('.rr__table thead th')[0].text()).toBe('Итог');
    expect(w.find('.rr__table tfoot').exists()).toBe(false);
    const row = w.findAll('.rr__table tbody tr')[0].findAll('td').map((td) => td.text());
    expect(row).toEqual(['Итого', '14']);
  });

  it('одиночная метрика (legacy rows): селектора метрик нет', async () => {
    const w = mountResult(aggStatus);
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();
    expect(w.find('[data-testid="rr-metric-value"]').exists()).toBe(false);
    expect(w.findComponent(barStub).props('data')).toEqual([{ label: 'Завершено', value: 5 }]);
  });

  it('после сброса в null и нового мультиметрик-отчёта график берёт первую метрику', async () => {
    const w = mountResult(aggMulti);
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await w.find('[data-testid="rr-metric-items_sum"]').trigger('click');
    await nextTick();
    expect(w.findComponent(barStub).props('seriesName')).toBe('Количество товаров');

    // Новый запуск: result -> null (пустой экран, вид сбрасывается на таблицу) ->
    // снова мультиметрика -> открываем график: метрика снова первая.
    await w.setProps({ result: null });
    await nextTick();
    await w.setProps({ result: aggMulti });
    await nextTick();
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();

    expect(w.findComponent(barStub).props('seriesName')).toBe('Количество заявок');
  });

  it('выбор Excel в меню зовёт экспорт с результатом, meta и форматом', async () => {
    exportSpy.mockClear();
    const meta = { period: { from: '2026-06-01', to: '2026-06-07' } };
    const w = mount(ReportResult, {
      props: { result: aggMulti, meta },
      global: { stubs: { AnalyticsAreaChart: areaStub, AnalyticsBarChart: barStub } },
    });
    await w.find('[data-testid="rr-export"]').trigger('click');
    await w.find('[data-testid="rr-export-excel"]').trigger('click');
    expect(exportSpy).toHaveBeenCalledWith(aggMulti, meta, 'excel');
  });

  it('выбор PDF в меню зовёт экспорт с форматом pdf', async () => {
    exportSpy.mockClear();
    const w = mount(ReportResult, {
      props: { result: aggMulti, meta: {} },
      global: { stubs: { AnalyticsAreaChart: areaStub, AnalyticsBarChart: barStub } },
    });
    await w.find('[data-testid="rr-export"]').trigger('click');
    await w.find('[data-testid="rr-export-pdf"]').trigger('click');
    expect(exportSpy).toHaveBeenCalledWith(aggMulti, {}, 'pdf');
  });

  it('кнопка выгрузки недоступна на пустом результате', () => {
    const w = mountResult({ ...aggStatus, rows: [], total: 0 });
    expect(w.find('[data-testid="rr-export"]').attributes('disabled')).toBeDefined();
  });

  it('list-режим тоже отдаёт кнопку выгрузки', () => {
    const w = mountResult(listRes);
    expect(w.find('[data-testid="rr-export"]').exists()).toBe(true);
  });

  it('падение экспорта эмитит export-error с сообщением', async () => {
    exportSpy.mockRejectedValueOnce(new Error('диск переполнен'));
    const w = mount(ReportResult, {
      props: { result: aggMulti },
      global: { stubs: { AnalyticsAreaChart: areaStub, AnalyticsBarChart: barStub } },
    });
    await w.find('[data-testid="rr-export"]').trigger('click');
    await w.find('[data-testid="rr-export-pdf"]').trigger('click');
    await nextTick();
    expect(w.emitted('export-error')[0]).toEqual(['диск переполнен']);
  });

  it('категориальный разрез (>=2 долей): тоггл Столбцы/Кольцо, кольцо рисует доли', async () => {
    const w = mountResult(aggMulti);
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();
    // По умолчанию столбцы.
    expect(w.findComponent(barStub).exists()).toBe(true);
    expect(w.find('[data-testid="rr-kind-donut"]').exists()).toBe(true);

    await w.find('[data-testid="rr-kind-donut"]').trigger('click');
    await nextTick();
    expect(w.findComponent(donutStub).exists()).toBe(true);
    expect(w.findComponent(barStub).exists()).toBe(false);
    expect(w.findComponent(donutStub).props('data')).toEqual([
      { label: 'ООО А', value: 10 }, { label: 'ООО Б', value: 4 },
    ]);
    expect(w.findComponent(donutStub).props('totalLabel')).toBe('Количество заявок');
  });

  it('период: тоггла Кольцо нет (доли по времени бессмысленны)', async () => {
    const w = mountResult(aggPeriod);
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();
    expect(w.find('[data-testid="rr-kind-donut"]').exists()).toBe(false);
  });

  it('один разрез (1 строка): тоггла Кольцо нет', async () => {
    const w = mountResult(aggStatus);
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();
    expect(w.find('[data-testid="rr-kind-donut"]').exists()).toBe(false);
  });

  it('смена разреза period->status переключает график area->столбцы', async () => {
    const w = mountResult(aggPeriod);
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();
    expect(w.findComponent(areaStub).exists()).toBe(true);

    await w.setProps({ result: aggStatus });
    await nextTick();
    // вид сохраняется (график), тип переключился на дефолт нового разреза
    expect(w.findComponent(areaStub).exists()).toBe(false);
    expect(w.findComponent(barStub).exists()).toBe(true);
  });
});

describe('ReportResult — длительности (#1240)', () => {
  const rowCells = (w, i) => w.findAll('.rr__table tbody tr')[i].findAll('td').map((td) => td.text());

  it('колонка type=duration: секунды -> читаемая длительность, а не сырое число', () => {
    const cells = rowCells(mountResult(aggDuration), 0);
    expect(cells[1]).toBe('2 ч 15 мин'); // 8100 секунд
    expect(cells[2]).toBe('3 сут'); // 259200 секунд
  });

  it('прочие метрики форматом длительности не задеты', () => {
    const cells = rowCells(mountResult(aggDuration), 0);
    expect(cells[3]).toBe('12,5'); // доля (float) — число
    expect(cells[4]).toBe('10'); // счётчик
  });

  it('нет ключа у длительности = «нет данных» -> «—», а не «0 мин»', () => {
    // Ключевая защита: у «ООО Б» этап завершения не пройден, движок ключ не выставил.
    // `?? 0` нарисовал бы «0 мин» = «завершилось мгновенно».
    const cells = rowCells(mountResult(aggDuration), 1);
    expect(cells[2]).toBe('—');
  });

  it('нет ключа у доли -> «—»: пустой бин не «отказов не было»', () => {
    expect(rowCells(mountResult(aggDuration), 1)[3]).toBe('—');
  });

  it('ноль длительности честен и остаётся «0 мин»', () => {
    // Движок COALESCE'ит пустое окно в 0 — это значение, а не отсутствие данных.
    expect(rowCells(mountResult(aggDuration), 1)[1]).toBe('0 с');
  });

  it('итоги: длительность форматируется, непосчитанная -> «—»', () => {
    const w = mountResult({
      ...aggDuration,
      totals: { avg_approval_time: 5400, applications_count: 14 },
      float_totals: {},
    });
    const totals = w.findAll('.rr__table tfoot td').map((td) => td.text());
    expect(totals[1]).toBe('1 ч 30 мин'); // 5400 секунд
    expect(totals[2]).toBe('—'); // завершения не было ни у кого
    expect(totals[3]).toBe('—'); // доля не посчитана
    expect(totals[4]).toBe('14'); // счётчик как был
  });

  it('счётчик без ключа по-прежнему 0 — для него это честно', () => {
    const w = mountResult({
      mode: 'aggregate',
      dimension: 'organization',
      columns: [{ key: 'applications_count', label: 'Количество заявок', unit: 'шт' }],
      metric_rows: [{ label: 'ООО Б', values: {} }],
      totals: {},
    });
    expect(rowCells(w, 0)[1]).toBe('0');
  });
});

describe('ReportResult — график длительностей (#1240)', () => {
  // Метрика графика выбирается тогглом; по умолчанию — первая колонка.
  async function openChart(w, metricKey) {
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();
    if (metricKey) {
      await w.find(`[data-testid="rr-metric-${metricKey}"]`).trigger('click');
      await nextTick();
    }
    return w;
  }

  it('duration-метрика: графику передан тип значения, ось и тултип форматирует он сам', async () => {
    const w = await openChart(mountResult(aggDuration));
    expect(w.findComponent(barStub).props('valueType')).toBe('duration');
  });

  it('обычная метрика: тип значения не передаётся', async () => {
    const w = await openChart(mountResult(aggDuration), 'applications_count');
    expect(w.findComponent(barStub).props('valueType')).toBe('');
  });

  it('непройденный этап уезжает в график как null, а не как 0', async () => {
    // Ключевая защита: у «ООО Б» завершения не было. `?? 0` дал бы столбец нулевой
    // высоты = «завершилось мгновенно», хотя таблица в этой же ячейке рисует «—».
    const w = await openChart(mountResult(aggDuration), 'avg_completion_time');
    expect(w.findComponent(barStub).props('data')).toEqual([
      { label: 'ООО А', value: 259200 },
      { label: 'ООО Б', value: null },
    ]);
  });

  it('duration-метрика: тоггла Кольцо нет (доли от суммы средних смысла не имеют)', async () => {
    const w = await openChart(mountResult(aggDuration));
    expect(w.find('[data-testid="rr-kind-donut"]').exists()).toBe(false);
  });

  it('переключение с duration на счётчик возвращает тоггл Кольцо', async () => {
    const w = await openChart(mountResult(aggDuration), 'applications_count');
    expect(w.find('[data-testid="rr-kind-donut"]').exists()).toBe(true);
  });
});

// Движок признака обрезки не отдаёт, поэтому «упёрлись в лимит» выводим сами:
// строк ровно столько, сколько разрешал запрос. Молчание тут стоило бы дорого -
// у разреза «период» обрезка съедает хвост периода, а не «лишние» строки.
describe('пометка обрезки по лимиту', () => {
  function mountWithLimit(result, limit) {
    return mount(ReportResult, {
      props: { result, limit },
      global: {
        stubs: { AnalyticsAreaChart: areaStub, AnalyticsBarChart: barStub, AnalyticsDonutChart: donutStub },
      },
    });
  }

  const rowsOf = (n) => ({
    mode: 'aggregate', dimension: 'period', unit: 'шт',
    rows: Array.from({ length: n }, (_, i) => ({ label: `2026-06-${i + 1}`, value: 1 })),
    total: n,
  });

  it('строк столько же, сколько лимит -> подпись есть', () => {
    const w = mountWithLimit(rowsOf(3), 3);
    expect(w.find('[data-testid="rr-truncated"]').text()).toContain('лимит 3');
  });

  it('строк меньше лимита -> подписи нет', () => {
    const w = mountWithLimit(rowsOf(2), 3);
    expect(w.find('[data-testid="rr-truncated"]').exists()).toBe(false);
  });

  it('лимит не передан -> подписи нет', () => {
    const w = mountWithLimit(rowsOf(3), 0);
    expect(w.find('[data-testid="rr-truncated"]').exists()).toBe(false);
  });

  it('выгрузка строк тоже помечается', () => {
    const w = mountWithLimit({ ...listRes, rows: [{ a: 'x' }, { a: 'y' }], total: 2 }, 2);
    expect(w.find('[data-testid="rr-truncated"]').exists()).toBe(true);
  });
});

// Мобильный адаптив (#1097 r3e). jsdom не считает layout, поэтому инварианты
// раскладки фиксируем чтением SFC: широкая таблица результата (произвольное число
// колонок) не должна рвать страницу - её держит горизонтальный скролл обёртки,
// а на телефоне ячейки уплотняются. Ловит откат этих правил при рефакторинге стилей.
describe('ReportResult — мобильный адаптив (guard)', () => {
  const sfc = readFileSync(resolve(__dirname, '../ReportResult.vue'), 'utf8');

  it('таблица завёрнута в горизонтальный скролл (страница не разъезжается на широком результате)', () => {
    // и aggregate, и list рендерят .rr__table-wrap; правило скролла одно на оба.
    expect(sfc).toMatch(/\.rr__table-wrap\s*\{[^}]*overflow-x:\s*auto/);
    expect(sfc).toContain('class="rr__table-wrap"');
  });

  it('на <=768 ячейки таблицы уплотняются (больше колонок до включения скролла)', () => {
    const media = sfc.slice(sfc.indexOf('@media (max-width: 768px)'));
    expect(media).toContain('@media (max-width: 768px)');
    // th и td получают компактный padding в мобильном блоке.
    expect(media).toMatch(/\.rr__table thead th\s*\{[^}]*padding:\s*8px 10px/);
    expect(media).toMatch(/\.rr__table tbody td[\s\S]*?padding:\s*8px 10px/);
  });
});

/*
 * Разбор результата на экране: полсотни строк иначе упорядочивали выгрузкой в Excel,
 * график сохраняли скриншотом (#2309).
 */
describe('ReportResult — разбор результата (#2309)', () => {
  const aggMany = {
    mode: 'aggregate',
    dimension: 'status',
    unit: 'шт',
    rows: [
      { label: 'Завершено', value: 3 },
      { label: 'Отказано', value: 11 },
      { label: 'В работе', value: 7 },
    ],
    total: 21,
  };

  const listRows = {
    mode: 'list',
    columns: [{ key: 'number', label: 'Номер' }, { key: 'mark', label: 'Марка' }],
    rows: [{ number: 'В 002', mark: 'Kia' }, { number: 'А 001', mark: 'BMW' }],
    total: 2,
  };

  function labels(wrapper) {
    return wrapper.findAll('tbody tr').map((r) => r.findAll('td')[0].text());
  }

  it('клик по заголовку сортирует сводку и переворачивает порядок повторным кликом', async () => {
    const wrapper = mountResult(aggMany);
    const valueHead = wrapper.findAll('th')[1];

    await valueHead.trigger('click');
    expect(labels(wrapper)).toEqual(['Завершено', 'В работе', 'Отказано']);
    expect(valueHead.attributes('aria-sort')).toBe('ascending');

    await valueHead.trigger('click');
    expect(labels(wrapper)).toEqual(['Отказано', 'В работе', 'Завершено']);
    expect(valueHead.attributes('aria-sort')).toBe('descending');
  });

  it('сортирует и выгрузку строк, по значениям колонки', async () => {
    const wrapper = mountResult(listRows);
    await wrapper.findAll('th')[0].trigger('click');
    expect(labels(wrapper)).toEqual(['А 001', 'В 002']);
  });

  it('новый результат сбрасывает сортировку - движок отдаёт свой порядок', async () => {
    const wrapper = mountResult(aggMany);
    await wrapper.findAll('th')[1].trigger('click');
    expect(labels(wrapper)[0]).toBe('Завершено');

    await wrapper.setProps({ result: { ...aggMany } });
    await nextTick();
    expect(labels(wrapper)).toEqual(['Завершено', 'Отказано', 'В работе']);
  });

  it('картинку предлагает только на графике и зовёт выгрузку холста', async () => {
    const wrapper = mountResult(aggPeriod);
    expect(wrapper.find('[data-testid="rr-export-png"]').exists()).toBe(false);

    await wrapper.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();
    await wrapper.find('[data-testid="rr-export"]').trigger('click');
    await nextTick();
    expect(wrapper.find('[data-testid="rr-export-png"]').exists()).toBe(true);

    await wrapper.find('[data-testid="rr-export-png"]').trigger('click');
    expect(pngSpy).toHaveBeenCalled();
    expect(exportSpy).not.toHaveBeenCalledWith(expect.anything(), expect.anything(), 'png');
  });

  it('линию отчёта не сглаживает: на дневном ряде с нулями кривая рисовала несуществующее', async () => {
    const wrapper = mountResult(aggPeriod);
    await wrapper.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();
    expect(wrapper.findComponent(areaStub).props('tension')).toBe(0);
  });
});
