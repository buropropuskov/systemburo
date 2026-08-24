import { describe, it, expect } from 'vitest';
import { SEARCH_TARGETS } from '../searchTargets';
import { OPEN_PARAM } from '@/utils/openQueryParam';

/**
 * Результат сквозного поиска ведёт к самой записи там, где страница это умеет:
 * строка поиска сужает список, id раскрывает карточку. Раздел без такой поддержки
 * по-прежнему открывается просто как раздел - это нормальное промежуточное
 * состояние, приёмники подключаются по одному.
 */
describe('SEARCH_TARGETS - куда ведёт результат', () => {
  it('сотрудник: и строка поиска, и открытие карточки', () => {
    expect(SEARCH_TARGETS.unique_employee.route(42, 'иванов')).toEqual({
      path: '/employeesview',
      query: { q: 'иванов', [OPEN_PARAM]: '42' },
    });
  });

  it('машина: то же самое', () => {
    expect(SEARCH_TARGETS.unique_car.route(7, 'а777')).toEqual({
      path: '/carsview',
      query: { q: 'а777', [OPEN_PARAM]: '7' },
    });
  });

  it('без id ведёт в раздел со строкой поиска - ссылка на список остаётся рабочей', () => {
    expect(SEARCH_TARGETS.unique_car.route(null, 'а777')).toEqual({
      path: '/carsview',
      query: { q: 'а777' },
    });
  });

  it('заявка открывается по своему id, как из уведомления', () => {
    expect(SEARCH_TARGETS.application.route(15, 'что-то')).toEqual({
      path: '/center',
      query: { open: '15' },
    });
  });

  it('раздел без приёмника ведёт просто в раздел, без пустых параметров', () => {
    expect(SEARCH_TARGETS.user.route(3, 'петров')).toEqual({ path: '/admin/users' });
  });
});
