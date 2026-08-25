/**
 * Клиентская сортировка списка заявок личного кабинета. Вынесена из
 * UserApplications.vue: блок script там сверх порога размера (build/check-file-sizes.js),
 * а сравнение самодостаточно и не трогает состояние компонента.
 *
 * Без выбранной колонки порядок хронологический - новые сверху; выбранная колонка
 * сортирует по своему значению в направлении direction. Исходный массив не мутируется.
 *
 * @param {object[]} applications
 * @param {?string} sortField колонка: application_number | sending_datetime | sender_name | confirmation | status
 * @param {'asc'|'desc'} [direction]
 * @returns {object[]}
 */
export function sortApplications(applications, sortField, direction = 'desc') {
  const sorted = [...applications];

  if (!sortField) {
    return sorted.sort((a, b) => new Date(b.sending_datetime) - new Date(a.sending_datetime));
  }

  return sorted.sort((a, b) => {
    const valueA = sortValue(a, sortField);
    const valueB = sortValue(b, sortField);
    if (valueA === null || valueB === null) return 0;
    if (valueA < valueB) return direction === 'asc' ? -1 : 1;
    if (valueA > valueB) return direction === 'asc' ? 1 : -1;
    return 0;
  });
}

// null - колонка неизвестна: порядок оставляем как есть.
function sortValue(application, sortField) {
  switch (sortField) {
    case 'application_number':
      return application.application_number;
    case 'sending_datetime':
      return new Date(application.sending_datetime);
    case 'sender_name':
      return application.sender_name || application.sender_full_name || '';
    case 'confirmation':
      return application.confirmation;
    case 'status':
      return application.status;
    default:
      return null;
  }
}
