/**
 * Готовые отчёты для галереи вкладки «Отчёты».
 *
 * Каждый пресет заполняет конструктор (ReportBuilder) и сразу строится. Ключи
 * метрик / разрезов / сущностей синхронизированы с whitelist-реестром бэка
 * (`internal/services/report_catalog.go`) — при изменении реестра сверить, иначе
 * пресет станет недоступен (см. presetAvailable). Период берётся из фильтра шапки,
 * пресеты его не задают.
 *
 * Данные наборов лежат в reportPresets.json - тот же файл копируется в
 * `internal/services/` и по нему бэкенд заводит системные шаблоны в базе.
 * Правится только JSON; после правки выполнить `make sync-presets`, иначе
 * Go-тест `TestReportPresets_FrontAndBackInSync` уронит сборку.
 *
 * @typedef {object} ReportPreset
 * @property {string} id
 * @property {string} title       краткое имя карточки
 * @property {string} description что считает
 * @property {string} resultHint  что будет в результате (столбцы/число)
 * @property {{ mode: 'aggregate'|'list', metric?: string, dimension?: string,
 *   granularity?: string, entity?: string }} form состояние конструктора
 */

import presets from './reportPresets.json';

/** @type {ReportPreset[]} */
export const REPORT_PRESETS = presets;

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
