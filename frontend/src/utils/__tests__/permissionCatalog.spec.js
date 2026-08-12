import { describe, expect, it } from 'vitest';

import {
  filterCatalog,
  flattenCatalog,
  parseTableKey,
  parseTablePermission,
} from '../permissionCatalog';

describe('flattenCatalog', () => {
  it('разворачивает детей следом за родителем', () => {
    const catalog = [
      { key: 'page.tables', children: [{ key: 'table.kpp4.view' }, { key: 'table.kpp4.entry' }] },
      { key: 'page.center' },
    ];
    expect(flattenCatalog(catalog).map((n) => n.key)).toEqual([
      'page.tables',
      'table.kpp4.view',
      'table.kpp4.entry',
      'page.center',
    ]);
  });

  it('пустой каталог даёт пустой список', () => {
    expect(flattenCatalog(undefined)).toEqual([]);
  });
});

describe('filterCatalog', () => {
  const catalog = [
    { key: 'page.center', display_name: 'Центр заявок', category: 'Навигация' },
    { key: 'page.cars', display_name: 'Автомобили', category: 'Навигация' },
    {
      key: 'page.admin',
      display_name: 'Администрирование',
      category: 'Администрирование',
      children: [{ key: 'page.admin.users', display_name: 'Пользователи' }],
    },
  ];

  it('пустой запрос возвращает каталог как есть', () => {
    expect(filterCatalog(catalog, '   ')).toBe(catalog);
  });

  it('ищет по имени, ключу и категории', () => {
    expect(filterCatalog(catalog, 'автом').map((n) => n.key)).toEqual(['page.cars']);
    expect(filterCatalog(catalog, 'page.center').map((n) => n.key)).toEqual(['page.center']);
    expect(filterCatalog(catalog, 'навигация').map((n) => n.key)).toEqual(['page.center', 'page.cars']);
  });

  it('родитель без совпадения остаётся ради подошедших детей', () => {
    const out = filterCatalog(catalog, 'пользоват');
    expect(out).toHaveLength(1);
    expect(out[0].key).toBe('page.admin');
    expect(out[0].children.map((c) => c.key)).toEqual(['page.admin.users']);
  });
});

describe('parseTableKey', () => {
  it('достаёт слаг и глагол, не сверяясь со словарём глаголов', () => {
    expect(parseTableKey('table.kpp_4.edit')).toEqual({ slug: 'kpp_4', verb: 'edit' });
    expect(parseTableKey('table.kpp.4.trash')).toEqual({ slug: 'kpp.4', verb: 'trash' });
  });

  it('ключ без слага или без глагола не разбирается', () => {
    expect(parseTableKey('table.view')).toBeNull();
    expect(parseTableKey('table.kpp_4.')).toBeNull();
    expect(parseTableKey('table..view')).toBeNull();
    expect(parseTableKey('page.cars')).toBeNull();
    expect(parseTableKey('')).toBeNull();
    expect(parseTableKey(undefined)).toBeNull();
  });
});

describe('parseTablePermission', () => {
  it('разбирает ключ и отрезает суффикс действия от имени таблицы', () => {
    expect(
      parseTablePermission({ key: 'table.kpp_4.export', display_name: 'КПП №4: Экспорт' }),
    ).toEqual({ slug: 'kpp_4', verb: 'export', verbTitle: 'Экспорт', tableName: 'КПП №4' });
  });

  it('имя таблицы со своей пунктуацией не режется по первому двоеточию', () => {
    const parsed = parseTablePermission({
      key: 'table.post_72.view',
      display_name: 'ПОСТ №72 (АВТО): въезд/выезд: Доступ к таблице',
    });
    expect(parsed.tableName).toBe('ПОСТ №72 (АВТО): въезд/выезд');
  });

  it('слаг с точкой внутри берёт глагол после последней точки', () => {
    const parsed = parseTablePermission({
      key: 'table.kpp.4.trash',
      display_name: 'КПП 4: Корзина',
    });
    expect(parsed).toMatchObject({ slug: 'kpp.4', verb: 'trash' });
  });

  // Имени таблицы и короткой подписи действия отсюда не следует; отнести право к
  // своей таблице всё равно можно -- по parseTableKey.
  it('незнакомый глагол не разбирается', () => {
    expect(parseTablePermission({ key: 'table.kpp_4.merge', display_name: 'КПП №4: Слияние' })).toBeNull();
  });

  it('несовпавший суффикс не разбирается', () => {
    expect(parseTablePermission({ key: 'table.kpp_4.export', display_name: 'КПП №4 - выгрузка' })).toBeNull();
  });

  it('не трогает права вне пространства таблиц', () => {
    expect(parseTablePermission({ key: 'page.cars', display_name: 'Автомобили' })).toBeNull();
    expect(parseTablePermission({ key: 'table.view', display_name: 'Что-то: Доступ к таблице' })).toBeNull();
    expect(parseTablePermission(null)).toBeNull();
  });
});
