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

  it('чёрный список: вместе с записью указывает вкладку - id у вкладок независимые', () => {
    expect(SEARCH_TARGETS.person_blacklist.route(5, 'иванов')).toEqual({
      path: '/admin/blacklist',
      query: { q: 'иванов', [OPEN_PARAM]: '5', tab: 'persons' },
    });
    expect(SEARCH_TARGETS.vehicle_blacklist.route(5, 'а777')).toEqual({
      path: '/admin/blacklist',
      query: { q: 'а777', [OPEN_PARAM]: '5', tab: 'vehicles' },
    });
  });

  it('пользователь: и строка поиска, и открытие карточки', () => {
    // Строка нужна не для поиска записи (список приходит целиком), а для «Показать
    // все»: без неё раздел открывается нефильтрованным.
    expect(SEARCH_TARGETS.user.route(3, 'петров')).toEqual({
      path: '/admin/users',
      query: { q: 'петров', [OPEN_PARAM]: '3' },
    });
  });

  it('раздел без приёмника ведёт просто в раздел, без пустых параметров', () => {
    // Объявление и документ открывать по ссылке нечем - см. отдельный кейс ниже.
    expect(SEARCH_TARGETS.announcement.route(3, 'объявление')).toEqual({ path: '/news' });
  });
});

/**
 * Справочники, обращения, новости и таблицы системы подключены к открытию записи
 * последними. У пяти справочников список приходит целиком, поэтому строку поиска в
 * адрес кладём для наглядности - найти запись помогает id.
 */
describe('SEARCH_TARGETS - справочники и остальные разделы', () => {
  const cases = [
    ['organization', '/admin/organizations'],
    ['company', '/admin/companies'],
    ['unload_place', '/admin/unload-places'],
    ['mark', '/admin/marks'],
    ['citizenship', '/admin/citizenship'],
    ['license_plate_format', '/admin/number-formats'],
    ['feedback', '/admin/feedback'],
  ];

  it.each(cases)('%s ведёт к записи со строкой поиска', (entity, path) => {
    expect(SEARCH_TARGETS[entity].route(4, 'ромашка')).toEqual({
      path,
      query: { q: 'ромашка', [OPEN_PARAM]: '4' },
    });
  });

  it('таблица системы: и строка поиска, и открытие', () => {
    expect(SEARCH_TARGETS.system_table.route(12, 'пропуска')).toEqual({
      path: '/table-constructor',
      query: { q: 'пропуска', [OPEN_PARAM]: '12' },
    });
  });

  it('новость открывается по id', () => {
    expect(SEARCH_TARGETS.news.route(5, 'ремонт')).toEqual({
      path: '/news',
      query: { [OPEN_PARAM]: '5' },
    });
  });

  it('объявления и документы по-прежнему ведут в раздел', () => {
    // Объявление на странице показывается только активное, документ по нажатию
    // скачивается - открывать по ссылке нечего, и это сознательно оставлено как есть.
    expect(SEARCH_TARGETS.announcement.route(5, 'ремонт')).toEqual({ path: '/news' });
    expect(SEARCH_TARGETS.document.route(5, 'инструкция')).toEqual({ path: '/news' });
  });
});
