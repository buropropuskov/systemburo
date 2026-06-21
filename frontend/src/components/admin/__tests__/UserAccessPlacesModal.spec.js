import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

import UserAccessPlacesModal from '../UserAccessPlacesModal.vue';
import { setUserUnloadPlaces, setUserTables } from '@/api/users';

const UNLOAD_PLACES = [
  { id: 1, name: 'Склад 1' },
  { id: 2, name: 'Склад 2' },
  { id: 3, name: 'Склад 3' },
];

// /system-tables отдаёт элементы в обёртке { table: {...} } поверх envelope -
// именно так, как боевой бэк (а не плоско). Раньше мок был плоским и пропустил баг
// "Без названия" в местах прохода.
const SYSTEM_TABLES = [
  { table: { id: 10, name: 'kpp_north', display_name: 'КПП Север', table_type: 'people' } },
  { table: { id: 11, name: 'garage', display_name: 'Гараж', table_type: 'cars' } },
  { table: { id: 12, name: 'kpp_south', display_name: 'КПП Юг', table_type: 'people' } },
];

const notify = vi.fn();

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn((path) => {
    if (path === '/unload-places') {
      return Promise.resolve({ ok: true, json: async () => UNLOAD_PLACES });
    }
    if (path === '/system-tables') {
      return Promise.resolve({ ok: true, json: async () => SYSTEM_TABLES });
    }
    return Promise.resolve({ ok: true, json: async () => [] });
  }),
}));

vi.mock('@/api/users', () => ({
  getUserUnloadPlaces: vi.fn(async () => [{ id: 2, name: 'Склад 2' }]),
  getUserTables: vi.fn(async () => [{ id: 12, display_name: 'КПП Юг', table_type: 'people' }]),
  setUserUnloadPlaces: vi.fn(async () => ({ ok: true })),
  setUserTables: vi.fn(async () => ({ ok: true })),
}));

vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify }),
}));

async function mountModal() {
  const wrapper = mount(UserAccessPlacesModal, {
    props: { user: { username: 'guard1' } },
    global: { stubs: { teleport: true } },
  });
  await flushPromises();
  return wrapper;
}

describe('UserAccessPlacesModal (#706 FE-S5)', () => {
  beforeEach(() => {
    notify.mockClear();
    setUserUnloadPlaces.mockClear();
    setUserTables.mockClear();
  });

  it('грузит пикеры, отфильтровывает cars-таблицы и предвыбирает места пользователя', async () => {
    const wrapper = await mountModal();

    expect(wrapper.vm.unloadPlaces).toHaveLength(3);
    // Гараж (cars) исключён - в местах прохода только people-таблицы.
    expect(wrapper.vm.tables.map(t => t.id)).toEqual([10, 12]);
    expect(wrapper.vm.selectedUnloadPlaceIds).toEqual([2]);
    expect(wrapper.vm.selectedTableIds).toEqual([12]);

    const items = wrapper.findAll('.place-item');
    expect(items).toHaveLength(5);
    expect(wrapper.findAll('.place-item.selected')).toHaveLength(2);

    // Места прохода показывают реальные имена из обёртки { table: {...} },
    // а не "Без названия" (баг до снятия double-wrap в fetchAllTables).
    const text = wrapper.text();
    expect(text).toContain('КПП Север');
    expect(text).toContain('КПП Юг');
    expect(text).not.toContain('Без названия');
  });

  it('Сохранить заблокировано без изменений и разблокируется после toggle', async () => {
    const wrapper = await mountModal();
    const sel = '[data-testid="access-places-save"]';
    expect(wrapper.find(sel).attributes('disabled')).toBeDefined();

    wrapper.vm.toggleUnloadPlace(1);
    await wrapper.vm.$nextTick();
    expect(wrapper.vm.isDirty).toBe(true);
    expect(wrapper.find(sel).attributes('disabled')).toBeUndefined();
  });

  it('сохранение шлёт выбранные id в оба эндпоинта, нотифицирует и эмитит updated', async () => {
    const wrapper = await mountModal();

    wrapper.vm.toggleUnloadPlace(1); // [2,1]
    wrapper.vm.toggleTable(10); // [12,10]
    await wrapper.vm.$nextTick();

    await wrapper.find('[data-testid="access-places-save"]').trigger('click');
    await flushPromises();

    expect(setUserUnloadPlaces).toHaveBeenCalledWith('guard1', [2, 1]);
    expect(setUserTables).toHaveBeenCalledWith('guard1', [12, 10]);
    expect(notify).toHaveBeenCalledWith(
      expect.objectContaining({ bold: 'guard1', type: 'success' }),
    );
    expect(wrapper.emitted('updated')).toHaveLength(1);
  });

  it('частичный сбой save: места разгрузки сохранены, места прохода — нет, dirty остаётся у упавшего', async () => {
    setUserTables.mockResolvedValueOnce({ ok: false });
    const wrapper = await mountModal();

    wrapper.vm.toggleUnloadPlace(1); // selected [2,1]
    wrapper.vm.toggleTable(10); // selected [12,10]
    await wrapper.vm.$nextTick();

    await wrapper.find('[data-testid="access-places-save"]').trigger('click');
    await flushPromises();

    // Места разгрузки прошли -> их original обновился; места прохода упали -> dirty.
    expect(wrapper.vm.originalUnloadPlaceIds).toEqual([2, 1]);
    expect(wrapper.vm.originalTableIds).toEqual([12]);
    expect(wrapper.vm.isDirty).toBe(true);
    expect(notify).toHaveBeenCalledWith(
      expect.objectContaining({ bold: 'места прохода', type: 'error' }),
    );
    // Модалка не закрылась при ошибке.
    expect(wrapper.emitted('close')).toBeUndefined();
  });

  it('toggle снимает уже выбранное место', async () => {
    const wrapper = await mountModal();
    wrapper.vm.toggleUnloadPlace(2); // снять предвыбранный
    await wrapper.vm.$nextTick();
    expect(wrapper.vm.selectedUnloadPlaceIds).toEqual([]);
  });
});
