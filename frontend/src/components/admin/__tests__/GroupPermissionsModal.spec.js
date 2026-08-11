import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import GroupPermissionsModal from '../GroupPermissionsModal.vue';

const catalog = [
  { key: 'page.cars', display_name: 'Автомобили', category: 'Навигация' },
  { key: 'page.admin.system_control', display_name: 'Техработы', category: 'Администрирование', super_only: true },
  { key: 'entity.cars.read', display_name: 'Автомобили: просмотр', category: 'Сотрудники и автомобили' },
  { key: 'table.kpp_4.view', display_name: 'КПП №4: Доступ к таблице', category: 'Таблицы' },
  { key: 'table.kpp_4.entry', display_name: 'КПП №4: Отметка въезда/входа', category: 'Таблицы' },
];

function mountModal(props = {}) {
  return mount(GroupPermissionsModal, {
    props: { show: true, catalog, initialKeys: ['page.cars'], ...props },
    global: { stubs: { teleport: true } },
  });
}

const toggle = (w, key) => w.find(`[data-key="${key}"] .tgl`);

describe('GroupPermissionsModal', () => {
  it('тумблеры отражают initialKeys', () => {
    const w = mountModal();
    expect(toggle(w, 'page.cars').attributes('aria-pressed')).toBe('true');
    expect(toggle(w, 'entity.cars.read').attributes('aria-pressed')).toBe('false');
  });

  it('super_only-право заблокировано и не добавляется по клику', async () => {
    const w = mountModal();
    const su = toggle(w, 'page.admin.system_control');
    expect(su.attributes('disabled')).toBeDefined();
    await su.trigger('click');
    await w.find('[data-testid="group-permissions-save"]').trigger('click');
    const saved = w.emitted('save')[0][0];
    expect(saved).not.toContain('page.admin.system_control');
  });

  it('включение права и сохранение эмитит обновлённый список ключей', async () => {
    const w = mountModal();
    await toggle(w, 'entity.cars.read').trigger('click');
    await w.find('[data-testid="group-permissions-save"]').trigger('click');
    const saved = w.emitted('save')[0][0];
    expect([...saved].sort()).toEqual(['entity.cars.read', 'page.cars']);
  });

  it('выключение уже выбранного права убирает его из сохранённого списка', async () => {
    const w = mountModal();
    await toggle(w, 'page.cars').trigger('click');
    await w.find('[data-testid="group-permissions-save"]').trigger('click');
    expect(w.emitted('save')[0][0]).toEqual([]);
  });

  it('поиск фильтрует по названию и по ключу', async () => {
    const w = mountModal();
    const search = w.find('[data-testid="group-permissions-search"]');
    await search.setValue('entity.cars');
    expect(toggle(w, 'entity.cars.read').exists()).toBe(true);
    expect(toggle(w, 'page.cars').exists()).toBe(false);
  });

  it('«выбрать все» по таблице кладёт её права в сохраняемый набор', async () => {
    const w = mountModal();
    const box = w.get('[data-table="kpp_4"]');
    expect(box.get('.ep-group__count').text()).toBe('0 из 2');

    await box.get('.ep-group__all').trigger('click');
    await w.find('[data-testid="group-permissions-save"]').trigger('click');
    expect([...w.emitted('save')[0][0]].sort()).toEqual([
      'page.cars',
      'table.kpp_4.entry',
      'table.kpp_4.view',
    ]);
  });
});
