/**
 * Готовые отчёты для галереи вкладки «Отчёты».
 *
 * Каждый пресет заполняет конструктор (ReportBuilder) и сразу строится. Ключи
 * метрик / разрезов / сущностей синхронизированы с whitelist-реестром бэка
 * (`internal/services/report_catalog.go`) — при изменении реестра сверить, иначе
 * пресет станет недоступен (см. presetAvailable). Период берётся из фильтра шапки,
 * пресеты его не задают.
 *
 * @typedef {object} ReportPreset
 * @property {string} id
 * @property {string} title       краткое имя карточки
 * @property {string} description что считает
 * @property {string} resultHint  что будет в результате (столбцы/число)
 * @property {{ mode: 'aggregate'|'list', metric?: string, dimension?: string,
 *   granularity?: string, entity?: string }} form состояние конструктора
 */

/** @type {ReportPreset[]} */
export const REPORT_PRESETS = [
  {
    id: 'summary',
    title: 'Сводка по заявкам',
    description: 'Динамика поданных заявок по дням за выбранный период.',
    resultHint: 'Таблица: дата + число заявок, с итогом.',
    form: { mode: 'aggregate', metric: 'applications_count', dimension: 'period', granularity: 'day' },
  },
  {
    id: 'work',
    title: 'Проведение работ',
    description: 'Заявки на работы с деталями: организация, наименование, ответственный, период и время работ, число людей.',
    resultHint: 'Таблица строк: 7 столбцов.',
    form: { mode: 'list', entity: 'work_applications' },
  },
  {
    id: 'cars_by_place',
    title: 'Машины по местам',
    description: 'Список машин с организацией, маркой и текущим местом разгрузки.',
    resultHint: 'Таблица строк: гос. номер, марка, организация, место, на территории.',
    form: { mode: 'list', entity: 'cars' },
  },
  {
    id: 'popular_places',
    title: 'Популярные места',
    description: 'Места разгрузки по числу въездов машин - самые загруженные сверху.',
    resultHint: 'Таблица: место + число въездов, с итогом.',
    form: { mode: 'aggregate', metric: 'car_entries_count', dimension: 'unload_place' },
  },
  {
    id: 'people_passages',
    title: 'Проходы людей',
    description: 'Входы людей по дням за выбранный период.',
    resultHint: 'Таблица: дата + число входов, с итогом.',
    form: { mode: 'aggregate', metric: 'people_entries_count', dimension: 'period', granularity: 'day' },
  },
  {
    id: 'applications_processing',
    title: 'Обработка заявок',
    description: 'Заявки в разрезе статусов обработки.',
    resultHint: 'Таблица: статус + число заявок, с итогом.',
    form: { mode: 'aggregate', metric: 'applications_count', dimension: 'status' },
  },
  {
    id: 'approval_time_trend',
    title: 'Скорость согласования',
    description: 'Среднее время от подачи заявки до её согласования по дням.',
    resultHint: 'Таблица: дата + среднее время согласования, с трендом.',
    form: { mode: 'aggregate', metric: 'avg_approval_time', dimension: 'period', granularity: 'day' },
  },
  {
    id: 'processing_time_by_org',
    title: 'Сроки обработки по организациям',
    description: 'Среднее время от подачи до принятия заявки в работу в разрезе организаций.',
    resultHint: 'Таблица: организация + среднее время обработки.',
    form: { mode: 'aggregate', metric: 'avg_processing_time', dimension: 'organization' },
  },
  {
    id: 'refusal_rate_trend',
    title: 'Доля отказов по дням',
    description: 'Динамика доли отклонённых и несогласованных заявок за период.',
    resultHint: 'Таблица: дата + доля отказов (%), с трендом.',
    form: { mode: 'aggregate', metric: 'refusal_rate', dimension: 'period', granularity: 'day' },
  },
  {
    id: 'approver_response_time',
    title: 'Время реакции согласующих',
    description: 'Среднее время ответа каждого согласующего - от назначения до голоса.',
    resultHint: 'Таблица: согласующий + среднее время реакции.',
    form: { mode: 'aggregate', metric: 'avg_approver_response_time', dimension: 'by_approver' },
  },
];

/**
 * Доступен ли пресет в текущем каталоге: для list — есть ли сущность, для
 * aggregate — есть ли метрика и поддерживает ли она выбранный разрез. Защищает
 * галерею от карточек, которые движок отклонит при рассинхроне с реестром бэка.
 *
 * @param {ReportPreset} preset
 * @param {object|null} catalog
 * @returns {boolean}
 */
export function presetAvailable(preset, catalog) {
  if (!catalog) return false;
  if (preset.form.mode === 'list') {
    return (catalog.list_entities || []).some((e) => e.key === preset.form.entity);
  }
  const metric = (catalog.metrics || []).find((m) => m.key === preset.form.metric);
  if (!metric) return false;
  return (metric.dimensions || []).includes(preset.form.dimension);
}
