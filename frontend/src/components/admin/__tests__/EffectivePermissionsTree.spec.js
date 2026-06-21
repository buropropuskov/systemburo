import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';

import EffectivePermissionsTree from '../EffectivePermissionsTree.vue';

const CATALOG = [
  { key: 'page.center', display_name: 'Центр заявок', category: 'Навигация' },
  { key: 'page.cars', display_name: 'Автомобили', category: 'Навигация' },
  { key: 'page.admin.system_control', display_name: 'Режим техработ', category: 'Администрирование', super_only: true },
];

const STATE = {
  'page.center': { on: true, source: 'role', locked: false },
  'page.cars': { on: false, source: null, locked: false },
  'page.admin.system_control': { on: false, source: null, locked: true },
};

function mountTree(overrides = {}) {
  return mount(EffectivePermissionsTree, {
    props: { catalog: CATALOG, stateByKey: STATE, ...overrides },
  });
}

describe('EffectivePermissionsTree', () => {
  it('группирует строки по категориям в порядке появления', () => {
    const wrapper = mountTree();
    const titles = wrapper.findAll('.ep-section__title').map((t) => t.text());
    expect(titles).toEqual(['Навигация', 'Администрирование']);
    expect(wrapper.findAll('.ep-row')).toHaveLength(3);
  });

  it('показывает бейдж источника только у включённых прав', () => {
    const wrapper = mountTree();
    const centerRow = wrapper.get('[data-key="page.center"]');
    expect(centerRow.find('.src--role').exists()).toBe(true);
    expect(centerRow.find('.src--role').text()).toBe('роль');

    // Выключенное право бейдж не показывает.
    const carsRow = wrapper.get('[data-key="page.cars"]');
    expect(carsRow.find('.src').exists()).toBe(false);
  });

  it('отражает on/off в классе тумблера', () => {
    const wrapper = mountTree();
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
    await tgl.trigger('click');
    expect(wrapper.emitted('toggle')).toBeUndefined();
  });

  it('рендерит дочерние узлы каталога как вложенные строки', () => {
    const catalog = [
      {
        key: 'page.tables',
        display_name: 'Таблицы',
        category: 'Навигация',
        children: [{ key: 'table.kpp4.view', display_name: 'КПП №4', category: 'Навигация' }],
      },
    ];
    const stateByKey = {
      'page.tables': { on: true, source: 'group', locked: false },
      'table.kpp4.view': { on: true, source: 'group', locked: false },
    };
    const wrapper = mountTree({ catalog, stateByKey });
    const child = wrapper.get('[data-key="table.kpp4.view"]');
    expect(child.classes()).toContain('ep-row--child');
    expect(child.find('.src--group').text()).toBe('группа');
  });
});
