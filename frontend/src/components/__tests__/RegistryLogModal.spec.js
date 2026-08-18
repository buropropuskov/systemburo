import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));

import RegistryLogModal from '../RegistryLogModal.vue';

const LOG = [
  {
    id: 3, unique_employee_id: 7, action_type: 'delete', comment: 'Сотрудник Пропавшев Игнат удалён из реестра',
    username: 'buropropuskov', user_last_name: 'Системный', user_first_name: 'Админ',
    created_at: '2026-08-17T20:30:00Z',
  },
  {
    id: 2, unique_employee_id: 7, action_type: 'data_changed', field_name: 'position',
    old_value: 'Слесарь', new_value: 'Монтажник', username: 'megobari', created_at: '2026-08-16T09:00:00Z',
  },
];

function mountModal(entity = 'employees') {
  return mount(RegistryLogModal, {
    props: { show: true, entity },
    global: { stubs: { teleport: true, BaseModal: { template: '<div><slot /></div>' }, BaseDropdown: true } },
  });
}

// Журнал заведён ради удалённых записей: у исчезнувшей строки ни карточки, ни истории по
// её номеру нет, поэтому «кем и когда удалена» отвечает только он.
describe('RegistryLogModal - журнал реестра', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
    apiRequest.mockResolvedValue({ ok: true, json: async () => LOG });
  });

  it('открытие грузит журнал сотрудников и показывает удаление с автором и временем', async () => {
    const wrapper = mountModal();
    await flushPromises();

    expect(apiRequest).toHaveBeenCalledWith('/unique-employees/history?limit=500');
    const rows = wrapper.findAll('.reglog__row');
    expect(rows).toHaveLength(2);
    expect(rows[0].text()).toContain('Пропавшев');
    expect(rows[0].text()).toContain('Системный Админ');
    expect(rows[0].text()).toContain('17.08.2026');
    expect(rows[0].classes()).toContain('reglog__row--delete');
  });

  it('правку без комментария описывает полем и значениями', async () => {
    const wrapper = mountModal();
    await flushPromises();

    expect(wrapper.findAll('.reglog__row')[1].text()).toContain('было «Слесарь», стало «Монтажник»');
  });

  it('машины грузятся своим endpoint', async () => {
    mountModal('cars');
    await flushPromises();

    expect(apiRequest).toHaveBeenCalledWith('/unique-cars/history?limit=500');
  });

  it('отбор по действию оставляет только удаления', async () => {
    const wrapper = mountModal();
    await flushPromises();

    await wrapper.setData({ actionFilter: 'delete' });
    const rows = wrapper.findAll('.reglog__row');
    expect(rows).toHaveLength(1);
    expect(rows[0].text()).toContain('удалён из реестра');
  });

  it('сбой загрузки показывает причину, а не пустой журнал', async () => {
    apiRequest.mockResolvedValue({ ok: false, status: 403, json: async () => ({}) });
    const wrapper = mountModal();
    await flushPromises();

    expect(wrapper.find('.reglog__state--error').text()).toContain('403');
    expect(wrapper.findAll('.reglog__row')).toHaveLength(0);
  });
});
