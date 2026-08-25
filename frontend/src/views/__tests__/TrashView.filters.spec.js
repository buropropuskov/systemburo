import {
  describe, it, expect, beforeEach, vi,
} from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import TrashView from '../TrashView.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import { apiRequest } from '@/api/client';
import { listTrash } from '@/api/trash';

// Фильтр организаций в корзине (#1398): мультивыбор через BaseDropdown, выбранные id
// уходят на бэк одним параметром. BaseDropdown намеренно не застаблен - проверяется
// вся проводка от клика по пункту меню до параметров запроса.

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
vi.mock('@/api/trash', () => ({
  listTrash: vi.fn(),
  restoreItems: vi.fn(),
  purgeItem: vi.fn(),
  clearTrash: vi.fn(),
}));

const okJson = (data) => ({ ok: true, json: async () => data });

const stubs = {
  teleport: true,
  RouterLink: true,
  RefreshButton: true,
  DateFilter: true,
  SearchComponent: true,
  VehicleDetailsModal: true,
  EmployeeDetailsModal: true,
  TrashHistoryModal: true,
  ConfirmationModal: true,
  ApplicationDetail: true,
};

function mockBackend() {
  apiRequest.mockImplementation((url) => {
    if (url === '/users/me') return Promise.resolve(okJson({ last_name: 'Иванов', first_name: 'Иван' }));
    if (url === '/organizations') {
      return Promise.resolve(okJson([
        { id: 1, name: 'Ромашка' },
        { id: 2, name: 'Василёк' },
      ]));
    }
    if (url.startsWith('/system-tables/name/')) {
      return Promise.resolve(okJson({ table: { id: 42, name: 'kpp4', table_type: 'cars', display_name: 'КПП 4' } }));
    }
    return Promise.resolve(okJson({}));
  });
  listTrash.mockResolvedValue([]);
}

async function mountTrash() {
  const wrapper = mount(TrashView, {
    global: {
      stubs,
      mocks: { $route: { params: { tableName: 'kpp4' } } },
    },
  });
  await flushPromises();
  return wrapper;
}

// Параметры последнего запроса корзины.
const lastParams = () => listTrash.mock.calls.at(-1)[1];

async function openMenu(wrapper) {
  const dropdown = wrapper.findComponent(BaseDropdown);
  await dropdown.find('.base-dropdown__button').trigger('click');
  return dropdown;
}

describe('TrashView - фильтр организаций', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setActivePinia(createPinia());
    mockBackend();
  });

  it('на старте грузит корзину без фильтра по организациям', async () => {
    await mountTrash();
    expect(listTrash).toHaveBeenCalledTimes(1);
    expect(listTrash.mock.calls[0][0]).toBe(42);
    expect(lastParams().organizationIds).toBeUndefined();
  });

  it('пункты меню берутся из справочника организаций', async () => {
    const wrapper = await mountTrash();
    const dropdown = await openMenu(wrapper);
    const labels = dropdown.findAll('.base-dropdown__item-text').map((n) => n.text());
    expect(labels).toEqual(['Ромашка', 'Василёк']);
    expect(dropdown.attributes('data-testid')).toBe('trash-filter-organizations');
  });

  it('выбор двух организаций уходит в запрос набором id', async () => {
    const wrapper = await mountTrash();
    const dropdown = await openMenu(wrapper);
    const items = dropdown.findAll('.base-dropdown__item');

    await items[0].trigger('click');
    await flushPromises();
    expect(lastParams().organizationIds).toEqual([1]);

    await items[1].trigger('click');
    await flushPromises();
    expect(lastParams().organizationIds).toEqual([1, 2]);
    expect(listTrash).toHaveBeenCalledTimes(3);
  });

  it('сброс выбора снимает фильтр, а не оставляет пустой набор', async () => {
    const wrapper = await mountTrash();
    const dropdown = await openMenu(wrapper);
    await dropdown.findAll('.base-dropdown__item')[0].trigger('click');
    await flushPromises();

    await dropdown.find('[data-testid="base-dropdown-clear"]').trigger('click');
    await flushPromises();

    expect(wrapper.vm.filters.organizationIds).toEqual([]);
    expect(lastParams().organizationIds).toBeUndefined();
  });

  it('подпись кнопки: плейсхолдер, имя одной организации, счётчик при нескольких', async () => {
    const wrapper = await mountTrash();
    const dropdown = await openMenu(wrapper);
    const buttonText = () => dropdown.find('.base-dropdown__text').text();
    expect(buttonText()).toBe('Все организации');

    const items = dropdown.findAll('.base-dropdown__item');
    await items[0].trigger('click');
    await flushPromises();
    expect(buttonText()).toBe('Ромашка');

    await items[1].trigger('click');
    await flushPromises();
    expect(buttonText()).toBe('Организация: 2');
  });
});
