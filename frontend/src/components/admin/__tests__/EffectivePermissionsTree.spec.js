import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';

import EffectivePermissionsTree from '../EffectivePermissionsTree.vue';
import { TABLE_VERB_ORDER, TABLE_VERB_TITLES } from '@/utils/permissionCatalog';

/*
 * Фикстура строится по размеру живого каталога, а не из пары прав: 55 статических
 * ключей и шесть таблиц по десять глаголов. На крошечном состоянии не видно ровно
 * того, ради чего заведена группировка (#1880) -- один пост занимал 20 строк
 * прокрутки, десять постов 200.
 */
const STATIC_CATEGORIES = [
  ['Навигация', 12],
  ['Шапка', 6],
  ['Центр заявок', 8],
  ['Карточка авто/сотрудника', 5],
  ['Сотрудники и автомобили', 7],
  ['Обзор и новости', 4],
  ['Администрирование', 13],
];

const TABLES = [
  ['kpp_4', 'КПП №4'],
  ['post_72', 'ПОСТ №72 (АВТО)'],
  ['prohodnaya', 'Проходная: главный вход'],
  ['sklad', 'Склад'],
  ['garazh', 'Гараж'],
  ['depo', 'Депо'],
];

/** Права одной таблицы -- по одному на каждый глагол, как их отдаёт бэкенд. */
function tableNodes(slug, name) {
  return TABLE_VERB_ORDER.map((verb) => ({
    key: `table.${slug}.${verb}`,
    display_name: `${name}: ${TABLE_VERB_TITLES[verb]}`,
    category: 'Таблицы',
  }));
}

function buildCatalog() {
  const nodes = [];
  // Именованные ключи, на которые смотрят соседние спеки и e2e.
  const named = {
    'Навигация': [
      { key: 'page.center', display_name: 'Центр заявок' },
      { key: 'page.cars', display_name: 'Автомобили' },
    ],
    'Администрирование': [
      { key: 'page.admin.system_control', display_name: 'Режим техработ', super_only: true },
    ],
  };
  STATIC_CATEGORIES.forEach(([category, count], catIndex) => {
    const fixed = named[category] || [];
    for (const node of fixed) nodes.push({ ...node, category });
    for (let i = fixed.length; i < count; i += 1) {
      nodes.push({ key: `page.c${catIndex}.${i}`, display_name: `Право ${category} ${i}`, category });
    }
  });
  for (const [slug, name] of TABLES) nodes.push(...tableNodes(slug, name));
  return nodes;
}

const CATALOG = buildCatalog();
const STATIC_TOTAL = STATIC_CATEGORIES.reduce((sum, [, n]) => sum + n, 0);

const OFF = { on: false, source: null, locked: false };

/** Состояния по умолчанию: всё выключено, кроме перечисленного в overrides. */
function buildState(catalog, overrides = {}) {
  const state = {};
  for (const node of catalog) {
    state[node.key] = overrides[node.key] || (node.super_only ? { ...OFF, locked: true } : OFF);
    for (const child of node.children || []) {
      state[child.key] = overrides[child.key] || OFF;
    }
  }
  return state;
}

function mountTree(overrides = {}) {
  const catalog = overrides.catalog || CATALOG;
  return mount(EffectivePermissionsTree, {
    props: {
      catalog,
      stateByKey: overrides.stateByKey || buildState(catalog),
      ...(overrides.expandAll === undefined ? {} : { expandAll: overrides.expandAll }),
    },
  });
}

/** Группа таблицы по слагу. */
const group = (w, slug) => w.get(`[data-table="${slug}"]`);

describe('EffectivePermissionsTree: секции и строки', () => {
  it('группирует строки по категориям в порядке появления', () => {
    const wrapper = mountTree();
    expect(wrapper.findAll('.ep-section__title').map((t) => t.text())).toEqual([
      ...STATIC_CATEGORIES.map(([name]) => name),
      'Таблицы',
    ]);
  });

  it('показывает бейдж источника только у включённых прав', () => {
    const wrapper = mountTree({
      stateByKey: buildState(CATALOG, { 'page.center': { on: true, source: 'role', locked: false } }),
    });
    const centerRow = wrapper.get('[data-key="page.center"]');
    expect(centerRow.find('.src--role').text()).toBe('роль');
    expect(wrapper.get('[data-key="page.cars"]').find('.src').exists()).toBe(false);
  });

  it('отражает on/off в классе тумблера', () => {
    const wrapper = mountTree({
      stateByKey: buildState(CATALOG, { 'page.center': { on: true, source: 'role', locked: false } }),
    });
    expect(wrapper.get('[data-key="page.center"]').find('.tgl').classes()).toContain('on');
    expect(wrapper.get('[data-key="page.cars"]').find('.tgl').classes()).not.toContain('on');
  });

  it('клик по тумблеру эмитит toggle с ключом', async () => {
    const wrapper = mountTree();
    await wrapper.get('[data-key="page.cars"]').find('.tgl').trigger('click');
    expect(wrapper.emitted('toggle')).toEqual([['page.cars']]);
  });

  it('super-only строка заблокирована, тумблер не эмитит и есть подпись', async () => {
    const wrapper = mountTree();
    const row = wrapper.get('[data-key="page.admin.system_control"]');
    expect(row.classes()).toContain('ep-row--locked');
    expect(row.find('small').text()).toContain('только Системный администратор');

    const tgl = row.find('.tgl');
    expect(tgl.classes()).toContain('locked');
    expect(tgl.attributes('aria-pressed')).toBe('false');
    await tgl.trigger('click');
    expect(wrapper.emitted('toggle')).toBeUndefined();
  });

  it('рендерит дочерние узлы каталога как вложенные строки', () => {
    const catalog = [
      {
        key: 'page.tables',
        display_name: 'Таблицы',
        category: 'Навигация',
        children: [{ key: 'entity.cars.read', display_name: 'Машины', category: 'Навигация' }],
      },
    ];
    const stateByKey = {
      'page.tables': { on: true, source: 'group', locked: false },
      'entity.cars.read': { on: true, source: 'group', locked: false },
    };
    const wrapper = mountTree({ catalog, stateByKey });
    const child = wrapper.get('[data-key="entity.cars.read"]');
    expect(child.classes()).toContain('ep-row--child');
    expect(child.find('.src--group').text()).toBe('группа');
  });
});

describe('EffectivePermissionsTree: второй уровень «Таблицы»', () => {
  it('права таблиц схлопываются в одну строку на таблицу', () => {
    const wrapper = mountTree();
    expect(wrapper.findAll('.ep-group')).toHaveLength(TABLES.length);

    // Все строки таблиц ушли внутрь групп: на верхнем уровне остались только
    // статические права (по одному на ключ, детей в фикстуре нет).
    const nested = wrapper.findAll('.ep-group__inner .ep-row');
    expect(nested).toHaveLength(TABLES.length * TABLE_VERB_ORDER.length);
    expect(wrapper.findAll('.ep-row')).toHaveLength(STATIC_TOTAL + nested.length);

    const verbRow = wrapper.get('[data-key="table.kpp_4.export"]');
    expect(verbRow.classes()).toContain('ep-row--verb');
    expect(verbRow.element.parentElement.classList.contains('ep-group__inner')).toBe(true);
  });

  it('внутри группы строка показывает только действие, полное имя остаётся в aria-label', () => {
    const wrapper = mountTree();
    const row = wrapper.get('[data-key="table.kpp_4.export"]');
    expect(row.get('.ep-row__label').text()).toBe('Экспорт');
    expect(row.get('.tgl').attributes('aria-label')).toBe('КПП №4: Экспорт');
  });

  it('на свёрнутой строке имя таблицы и счётчик «N из M»', () => {
    const wrapper = mountTree({
      stateByKey: buildState(CATALOG, {
        'table.kpp_4.view': { on: true, source: 'override', locked: false },
        'table.kpp_4.entry': { on: true, source: 'override', locked: false },
        'table.kpp_4.exit': { on: true, source: 'group', locked: true },
      }),
    });
    const kpp = group(wrapper, 'kpp_4');
    expect(kpp.get('.ep-group__name').text()).toBe('КПП №4');
    // Заблокированное право приходит из группы и действует -- считаем выданным.
    expect(kpp.get('.ep-group__count').text()).toBe('3 из 10');
    expect(group(wrapper, 'sklad').get('.ep-group__count').text()).toBe('0 из 10');
  });

  it('таблицы свёрнуты по умолчанию, статические категории -- нет', () => {
    const wrapper = mountTree();
    for (const [slug] of TABLES) {
      const toggle = group(wrapper, slug).get('.ep-group__toggle');
      expect(toggle.attributes('aria-expanded')).toBe('false');
      expect(group(wrapper, slug).get('.ep-group__body').classes()).not.toContain('ep-group__body--open');
    }
    // Строка статической категории видна сразу, без раскрытия.
    expect(wrapper.get('[data-key="page.cars"]').exists()).toBe(true);
  });

  it('клик по заголовку раскрывает и сворачивает таблицу', async () => {
    const wrapper = mountTree();
    const toggle = group(wrapper, 'kpp_4').get('.ep-group__toggle');

    await toggle.trigger('click');
    expect(toggle.attributes('aria-expanded')).toBe('true');
    expect(group(wrapper, 'kpp_4').get('.ep-group__body').classes()).toContain('ep-group__body--open');
    // Соседняя таблица осталась свёрнутой.
    expect(group(wrapper, 'sklad').get('.ep-group__toggle').attributes('aria-expanded')).toBe('false');

    await toggle.trigger('click');
    expect(toggle.attributes('aria-expanded')).toBe('false');
  });

  it('поиск раскрывает совпадение внутри свёрнутой таблицы', async () => {
    const wrapper = mountTree();
    expect(group(wrapper, 'kpp_4').get('.ep-group__toggle').attributes('aria-expanded')).toBe('false');

    // Потребители включают expandAll на время поиска.
    await wrapper.setProps({ expandAll: true });
    for (const [slug] of TABLES) {
      expect(group(wrapper, slug).get('.ep-group__toggle').attributes('aria-expanded')).toBe('true');
      expect(group(wrapper, slug).get('.ep-group__body').classes()).toContain('ep-group__body--open');
    }
  });
});

describe('EffectivePermissionsTree: «выбрать все» по таблице', () => {
  it('включает только выключенные и не трогает заблокированные', async () => {
    const wrapper = mountTree({
      stateByKey: buildState(CATALOG, {
        'table.kpp_4.view': { on: true, source: 'override', locked: false },
        'table.kpp_4.entry': { on: true, source: 'group', locked: true },
        'table.kpp_4.exit': { on: false, source: null, locked: true },
      }),
    });
    const btn = group(wrapper, 'kpp_4').get('.ep-group__all');
    expect(btn.text()).toBe('Выбрать все');

    await btn.trigger('click');
    const keys = wrapper.emitted('toggle').map(([key]) => key);
    expect(keys).not.toContain('table.kpp_4.entry');
    expect(keys).not.toContain('table.kpp_4.exit');
    expect(keys).not.toContain('table.kpp_4.view');
    expect(keys).toEqual(
      TABLE_VERB_ORDER.filter((v) => !['view', 'entry', 'exit'].includes(v)).map(
        (v) => `table.kpp_4.${v}`,
      ),
    );
  });

  it('на полном наборе снимает все доступные права таблицы', async () => {
    const overrides = {};
    for (const verb of TABLE_VERB_ORDER) {
      overrides[`table.sklad.${verb}`] = { on: true, source: 'override', locked: false };
    }
    overrides['table.sklad.delete'] = { on: true, source: 'group', locked: true };

    const wrapper = mountTree({ stateByKey: buildState(CATALOG, overrides) });
    const btn = group(wrapper, 'sklad').get('.ep-group__all');
    expect(btn.text()).toBe('Снять все');

    await btn.trigger('click');
    const keys = wrapper.emitted('toggle').map(([key]) => key);
    expect(keys).toHaveLength(TABLE_VERB_ORDER.length - 1);
    expect(keys).not.toContain('table.sklad.delete');
  });

  it('кнопка не рисуется, когда переключать нечего', () => {
    const overrides = {};
    for (const verb of TABLE_VERB_ORDER) {
      overrides[`table.depo.${verb}`] = { on: true, source: 'group', locked: true };
    }
    const wrapper = mountTree({ stateByKey: buildState(CATALOG, overrides) });
    const depo = group(wrapper, 'depo');
    expect(depo.find('.ep-group__all').exists()).toBe(false);
    expect(depo.get('.ep-group__count').text()).toBe('10 из 10');
  });
});

/*
 * Право с глаголом вне словаря -- не выдумка: в базе живут legacy `table.<slug>.edit`
 * с подписью чужого формата («Редактирование таблицы kpp_4»). Раньше такое право
 * висело отдельной строкой рядом со своей же таблицей, и владелец прочитал её как
 * поломку. Терять права нельзя, поэтому строка не выбрасывается, а уходит в группу
 * по слагу из ключа.
 */
describe('EffectivePermissionsTree: глагол вне словаря', () => {
  const legacyEdit = {
    key: 'table.kpp_4.edit',
    display_name: 'Редактирование таблицы kpp_4',
    category: 'Таблицы',
  };

  it('уходит в группу своей таблицы с подписью от бэкенда', () => {
    const catalog = [...tableNodes('kpp_4', 'КПП №4'), legacyEdit];
    const wrapper = mountTree({ catalog, stateByKey: buildState(catalog) });

    expect(wrapper.findAll('.ep-group')).toHaveLength(1);
    const row = wrapper.get('[data-key="table.kpp_4.edit"]');
    expect(row.classes()).toContain('ep-row--verb');
    expect(row.element.parentElement.classList.contains('ep-group__inner')).toBe(true);
    // Короткой подписи действия для незнакомого глагола нет -- показываем то
    // единственное, что о нём известно.
    expect(row.get('.ep-row__label').text()).toBe('Редактирование таблицы kpp_4');
    expect(group(wrapper, 'kpp_4').get('.ep-group__name').text()).toBe('КПП №4');
  });

  it('входит в знаменатель: счёт по фактическим правам, а не по длине словаря глаголов', () => {
    const catalog = [...tableNodes('kpp_4', 'КПП №4'), legacyEdit];
    const wrapper = mountTree({
      catalog,
      stateByKey: buildState(catalog, {
        'table.kpp_4.view': { on: true, source: 'override', locked: false },
        'table.kpp_4.edit': { on: true, source: 'role', locked: true },
      }),
    });
    const kpp = group(wrapper, 'kpp_4');
    expect(kpp.get('.ep-group__count').text()).toBe(`2 из ${TABLE_VERB_ORDER.length + 1}`);
    expect(kpp.get('.ep-group__toggle').attributes('aria-label')).toBe(
      `КПП №4: выдано 2 из ${TABLE_VERB_ORDER.length + 1}`,
    );
  });

  it('«выбрать все» переключает и его', async () => {
    const catalog = [...tableNodes('kpp_4', 'КПП №4'), legacyEdit];
    const wrapper = mountTree({ catalog, stateByKey: buildState(catalog) });

    await group(wrapper, 'kpp_4').get('.ep-group__all').trigger('click');
    expect(wrapper.emitted('toggle').map(([key]) => key)).toContain('table.kpp_4.edit');
  });

  it('таблица из одних неразобранных прав озаглавлена слагом', () => {
    const catalog = [
      { key: 'table.kpp_4.export', display_name: 'Экспорт КПП №4', category: 'Таблицы' },
      legacyEdit,
    ];
    const wrapper = mountTree({ catalog, stateByKey: buildState(catalog) });
    // Живого имени взять неоткуда: подпись ни одного права не разобралась.
    expect(group(wrapper, 'kpp_4').get('.ep-group__name').text()).toBe('kpp_4');
    expect(group(wrapper, 'kpp_4').get('.ep-group__count').text()).toBe('0 из 2');
  });
});

describe('EffectivePermissionsTree: что в группу не попадает', () => {
  it('право без разбираемого слага остаётся строкой верхнего уровня', () => {
    const catalog = [
      { key: 'table.view', display_name: 'Таблицы: Доступ', category: 'Таблицы' },
      { key: 'table.kpp_4.', display_name: 'КПП №4: хвост', category: 'Таблицы' },
      { key: 'page.cars', display_name: 'Автомобили', category: 'Навигация' },
    ];
    const wrapper = mountTree({ catalog, stateByKey: buildState(catalog) });

    expect(wrapper.findAll('.ep-group')).toHaveLength(0);
    for (const key of ['table.view', 'table.kpp_4.', 'page.cars']) {
      const row = wrapper.get(`[data-key="${key}"]`);
      expect(row.classes()).not.toContain('ep-row--verb');
      expect(row.element.parentElement.classList.contains('ep-group__inner')).toBe(false);
    }
  });

  it('пустой каталог даёт заглушку', () => {
    const wrapper = mountTree({ catalog: [], stateByKey: {} });
    expect(wrapper.get('.ep-empty').text()).toBe('Нет доступных прав');
  });
});
