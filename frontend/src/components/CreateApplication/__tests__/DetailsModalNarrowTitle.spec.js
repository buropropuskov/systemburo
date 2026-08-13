import { describe, it, expect } from 'vitest';

import EmployeeDetailsModal from '../EmployeeDetailsModal.vue';
import VehicleDetailsModal from '../VehicleDetailsModal.vue';

/**
 * Заголовок шапки модалки деталей на узком экране.
 *
 * Длинный вариант («Детальная информация о сотруднике») занимал строку целиком, отжимал
 * крестик и переносился на вторую строку - владелец описал это как «не вмещается в одну
 * строку, переносится». Кнопки в шапке видны не всегда, поэтому короткий вариант,
 * подобранный по их числу, на телефоне не срабатывал: без кнопок брался самый длинный.
 *
 * Ширину считает `useNarrowScreen`, поэтому проверяем сам выбор строки, подставляя признак
 * напрямую - jsdom медиазапросы не вычисляет.
 */
function title(component, { isNarrow, visibleActionsCount }) {
  return component.computed.modalTitle.call({ isNarrow, visibleActionsCount });
}

describe('Модалки деталей — заголовок шапки на телефоне', () => {
  const cases = [
    ['сотрудника', EmployeeDetailsModal],
    ['Т/С', VehicleDetailsModal],
  ];

  it.each(cases)('модалка %s: на узком экране заголовок короткий при любом числе кнопок', (_name, component) => {
    for (const visibleActionsCount of [0, 1, 2]) {
      expect(title(component, { isNarrow: true, visibleActionsCount })).toBe('Информация');
    }
  });

  it.each(cases)('модалка %s: на широком экране длинный заголовок сохраняется', (_name, component) => {
    const wide = title(component, { isNarrow: false, visibleActionsCount: 0 });

    expect(wide.startsWith('Детальная информация')).toBe(true);
    expect(title(component, { isNarrow: false, visibleActionsCount: 2 })).toBe('Информация');
  });
});
