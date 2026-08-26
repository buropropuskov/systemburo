import { describe, it, expect, vi } from 'vitest';
import { readOpenIdFromRoute, clearOpenFromRoute, openItemFromRoute, registryScopeForRoute, OPEN_PARAM } from '../openQueryParam';

/**
 * Переход из сквозного поиска обязан приводить к самой записи, а не к разделу, где её
 * ещё надо искать глазами. `?q` сужает список, `?open` раскрывает карточку.
 */

function routeWith(query) {
  return { query };
}

function router() {
  return { replace: vi.fn().mockResolvedValue(undefined) };
}

describe('readOpenIdFromRoute', () => {
  it('читает id из адреса', () => {
    expect(readOpenIdFromRoute(routeWith({ [OPEN_PARAM]: '42' }))).toBe(42);
  });

  it('мусор и отсутствие параметра не считаются идентификатором', () => {
    expect(readOpenIdFromRoute(routeWith({}))).toBeNull();
    expect(readOpenIdFromRoute(routeWith({ [OPEN_PARAM]: 'абв' }))).toBeNull();
    // 0 и отрицательные id записей не бывают - такой адрес битый.
    expect(readOpenIdFromRoute(routeWith({ [OPEN_PARAM]: '0' }))).toBeNull();
    expect(readOpenIdFromRoute(routeWith({ [OPEN_PARAM]: '-3' }))).toBeNull();
    expect(readOpenIdFromRoute(undefined)).toBeNull();
  });
});

describe('clearOpenFromRoute', () => {
  it('убирает open, сохраняя остальные параметры', () => {
    const r = router();
    clearOpenFromRoute(r, routeWith({ [OPEN_PARAM]: '42', q: 'иванов', archive: 'true' }));

    expect(r.replace).toHaveBeenCalledWith({ query: { q: 'иванов', archive: 'true' } });
  });

  it('без open в адресе навигации не делает - лишняя replace ломала бы историю', () => {
    const r = router();
    clearOpenFromRoute(r, routeWith({ q: 'иванов' }));

    expect(r.replace).not.toHaveBeenCalled();
  });
});

describe('openItemFromRoute', () => {
  const items = [{ id: 1 }, { id: 42 }, { id: 7 }];

  it('открывает запись из списка и вычищает open из адреса', () => {
    const open = vi.fn();
    const r = router();

    const done = openItemFromRoute({ router: r, route: routeWith({ [OPEN_PARAM]: '42', q: 'а777' }), items, open });

    expect(done).toBe(true);
    expect(open).toHaveBeenCalledWith({ id: 42 });
    expect(r.replace).toHaveBeenCalledWith({ query: { q: 'а777' } });
  });

  it('id в адресе строкой, в списке числом - сравниваем значения, а не типы', () => {
    const open = vi.fn();
    openItemFromRoute({ router: router(), route: routeWith({ [OPEN_PARAM]: '7' }), items, open });

    expect(open).toHaveBeenCalledWith({ id: 7 });
  });

  it('записи ещё нет в загруженном - open остаётся в адресе для следующей попытки', () => {
    const open = vi.fn();
    const r = router();

    const done = openItemFromRoute({ router: r, route: routeWith({ [OPEN_PARAM]: '999' }), items, open });

    expect(done).toBe(false);
    expect(open).not.toHaveBeenCalled();
    expect(r.replace).not.toHaveBeenCalled();
  });

  it('без параметра ничего не открывает', () => {
    const open = vi.fn();
    expect(openItemFromRoute({ router: router(), route: routeWith({ q: 'а777' }), items, open })).toBe(false);
    expect(open).not.toHaveBeenCalled();
  });

  it('пустой список не роняет обход', () => {
    const open = vi.fn();
    expect(openItemFromRoute({ router: router(), route: routeWith({ [OPEN_PARAM]: '1' }), items: undefined, open })).toBe(false);
  });
});

describe('registryScopeForRoute', () => {
  const all = () => true;
  const none = () => false;
  const only = (granted) => (p) => p === granted;

  it('переход из поиска открывает самую широкую доступную область', () => {
    // Иначе найденная чужая запись не попадает в «Мои», и открывать нечего.
    expect(registryScopeForRoute(routeWith({ [OPEN_PARAM]: '5' }), all)).toBe('all_system');
  });

  it('без права на всю систему берётся следующая по ширине', () => {
    expect(registryScopeForRoute(routeWith({ [OPEN_PARAM]: '5' }), only('section.registry.organization'))).toBe('organization');
    expect(registryScopeForRoute(routeWith({ [OPEN_PARAM]: '5' }), only('section.registry.company'))).toBe('company');
  });

  it('без прав на чужие записи область остаётся своей', () => {
    expect(registryScopeForRoute(routeWith({ [OPEN_PARAM]: '5' }), none)).toBe('user');
  });

  it('обычный заход без open область не трогает', () => {
    expect(registryScopeForRoute(routeWith({}), all)).toBe('user');
    expect(registryScopeForRoute(routeWith({ q: 'иванов' }), all)).toBe('user');
  });
});

describe('openItemFromRoute: запись-обёртка', () => {
  // Список таблиц системы состоит из обёрток {table: {...}} - собственного id у строки нет.
  const wrapped = [{ table: { id: 3 } }, { table: { id: 8 } }];

  it('находит запись по вложенному id', () => {
    const open = vi.fn();
    const done = openItemFromRoute({
      router: router(), route: routeWith({ [OPEN_PARAM]: '8' }), items: wrapped, open,
      idOf: (row) => row?.table?.id,
    });

    expect(done).toBe(true);
    expect(open).toHaveBeenCalledWith({ table: { id: 8 } });
  });

  it('без idOf такой список не откроется - у строки нет своего id', () => {
    const open = vi.fn();
    expect(openItemFromRoute({ router: router(), route: routeWith({ [OPEN_PARAM]: '8' }), items: wrapped, open })).toBe(false);
  });
});

describe('openItemFromRoute: списки-обёртки в справочниках', () => {
  // Форматы номеров лежат как {format: {...}}, таблицы системы - как {table: {...}}.
  it('формат номера находится по вложенному id', () => {
    const open = vi.fn();
    const rows = [{ format: { id: 4 } }, { format: { id: 11 } }];

    openItemFromRoute({
      router: router(), route: routeWith({ [OPEN_PARAM]: '11' }), items: rows, open,
      idOf: (row) => row?.format?.id,
    });

    expect(open).toHaveBeenCalledWith({ format: { id: 11 } });
  });
});
